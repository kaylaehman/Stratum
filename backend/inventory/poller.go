package inventory

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/nodeconn"
)

// DefaultInterval is the base poll interval per node.
const DefaultInterval = 30 * time.Second

// CycleMessage is broadcast once per node per poll cycle on topic
// "tree:<nodeId>". It carries the cycle's seq and any deltas (possibly empty —
// it doubles as a heartbeat so clients can detect a seq gap and resync).
type CycleMessage struct {
	NodeID string  `json:"node_id"`
	Seq    uint64  `json:"seq"`
	Deltas []Delta `json:"deltas"`
}

// Poller refreshes inventory for every node on an interval and broadcasts
// deltas over the hub. One poll per node runs at a time (skip-if-busy).
type Poller struct {
	store    db.Store
	conn     *nodeconn.Manager
	hub      *hub.Hub
	seq      *seqRegistry
	logger   *slog.Logger
	interval time.Duration

	mu       sync.Mutex
	inflight map[string]bool
}

// NewPoller constructs a Poller.
func NewPoller(store db.Store, conn *nodeconn.Manager, h *hub.Hub, logger *slog.Logger) *Poller {
	return &Poller{
		store:    store,
		conn:     conn,
		hub:      h,
		seq:      newSeqRegistry(),
		logger:   logger,
		interval: DefaultInterval,
		inflight: map[string]bool{},
	}
}

// CurrentSeq returns the latest broadcast seq for a node (for GET /api/tree).
func (p *Poller) CurrentSeq(nodeID string) uint64 { return p.seq.current(nodeID) }

// Run polls until ctx is cancelled. It re-reads the node table each tick so
// added/removed nodes are picked up without a restart.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	p.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

func (p *Poller) tick(ctx context.Context) {
	nodes, err := p.store.ListNodes(ctx)
	if err != nil {
		p.logger.Warn("inventory: list nodes", "error", err)
		return
	}
	for _, n := range nodes {
		if !p.acquire(n.ID) {
			continue // a poll for this node is still running — skip (no pile-up)
		}
		go func(n db.Node) {
			defer p.release(n.ID)
			// Small jitter to avoid a thundering herd across nodes.
			time.Sleep(time.Duration(rand.Int63n(int64(2 * time.Second))))
			p.pollNode(ctx, n)
		}(n)
	}
}

func (p *Poller) pollNode(ctx context.Context, n db.Node) {
	caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
	var env struct {
		ProxmoxAuthStatus string `json:"proxmox_auth_status"`
	}
	_ = json.Unmarshal([]byte(n.CapabilitiesJSON), &env)

	clients, err := p.conn.Get(ctx, n.ID)
	if err != nil {
		p.logger.Warn("inventory: get clients", "node", n.ID, "error", err)
		return
	}

	var deltas []Delta
	reachable := false

	if caps.Docker && clients.Docker != nil {
		if cs, err := enumDocker(ctx, clients.Docker, n.ID); err == nil {
			reachable = true
			if d, err := reconcileContainers(ctx, p.store, n.ID, cs); err == nil {
				deltas = append(deltas, d...)
			}
		}
	}
	if caps.Proxmox && env.ProxmoxAuthStatus == "confirmed" && clients.Proxmox != nil {
		if vms, err := enumProxmox(ctx, clients.Proxmox, n.ID); err == nil {
			reachable = true
			if d, err := reconcileVMs(ctx, p.store, n.ID, vms); err == nil {
				deltas = append(deltas, d...)
			}
		}
	}

	p.updateNodeStatus(ctx, n, reachable)

	seq := p.seq.next(n.ID)
	for i := range deltas {
		deltas[i].Seq = seq
	}
	msg, err := json.Marshal(CycleMessage{NodeID: n.ID, Seq: seq, Deltas: deltas})
	if err == nil {
		p.hub.Broadcast("tree:"+n.ID, msg)
	}
}

func (p *Poller) updateNodeStatus(ctx context.Context, n db.Node, reachable bool) {
	newStatus := "unreachable"
	if reachable {
		newStatus = "ok"
		now := time.Now()
		n.LastSeen = &now
	}
	if n.Status == newStatus && !reachable {
		return // avoid a write storm on a persistently-down node
	}
	n.Status = newStatus
	if err := p.store.UpdateNode(ctx, n); err != nil {
		p.logger.Warn("inventory: update node status", "node", n.ID, "error", err)
	}
}

func (p *Poller) acquire(nodeID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inflight[nodeID] {
		return false
	}
	p.inflight[nodeID] = true
	return true
}

func (p *Poller) release(nodeID string) {
	p.mu.Lock()
	delete(p.inflight, nodeID)
	p.mu.Unlock()
}
