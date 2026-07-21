package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/KAE-Labs/stratum/backend/capabilities"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/hub"
	"github.com/KAE-Labs/stratum/backend/nodeconn"
)

// NotifyFunc is called when a container OOM/health event is detected.
// Signature matches the SetNotify pattern used across services.
type NotifyFunc func(ctx context.Context, trigger, title, text string)

// trigger key constants — kept local to avoid importing the webhooks package.
const (
	triggerContainerOOM       = "container.oom"
	triggerContainerUnhealthy = "container.unhealthy"
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

// ReachabilityFunc probes a node's reachability (typically an SSH+Docker+Proxmox
// probe with the pinned host key) and returns whether it's reachable plus a
// sanitized error category when it isn't. Injected so the poller can confirm
// SSH-reachable nodes without the inventory package depending on the SSH/probe
// stack directly.
// ReachabilityFunc returns a status ("ok" | "degraded" | "unreachable" |
// "unknown") and a sanitized last-error. "degraded" means a management API is up
// but SSH (host-level ops) is down.
type ReachabilityFunc func(ctx context.Context, n db.Node) (status, lastErr string)

// Poller refreshes inventory for every node on an interval and broadcasts
// deltas over the hub. One poll per node runs at a time (skip-if-busy).
type Poller struct {
	store    db.Store
	conn     *nodeconn.Manager
	hub      *hub.Hub
	seq      *seqRegistry
	logger   *slog.Logger
	interval time.Duration
	reach    ReachabilityFunc
	notify   NotifyFunc // may be nil; OOM/health notifications are best-effort

	// enumContainers enumerates a node's containers from a Docker client. It is a
	// field (defaulting to enumDocker) only so tests can inject a fake lister and
	// prove Docker enumeration runs independent of the SSH/reachability result.
	enumContainers func(ctx context.Context, cl containerLister, nodeID string) ([]Delta, error)

	mu       sync.Mutex
	inflight map[string]bool
}

// SetReachability installs the SSH/probe-based reachability fallback. Without
// it, a node is reachable only if its Docker or Proxmox enumeration succeeds.
func (p *Poller) SetReachability(f ReachabilityFunc) { p.reach = f }

// SetNotify wires a notification callback fired on container OOM-kill or
// unhealthy-healthcheck transitions. If nil, those notifications are skipped.
func (p *Poller) SetNotify(fn NotifyFunc) { p.notify = fn }

// NewPoller constructs a Poller.
func NewPoller(store db.Store, conn *nodeconn.Manager, h *hub.Hub, logger *slog.Logger) *Poller {
	p := &Poller{
		store:    store,
		conn:     conn,
		hub:      h,
		seq:      newSeqRegistry(),
		logger:   logger,
		interval: DefaultInterval,
		inflight: map[string]bool{},
	}
	p.enumContainers = p.defaultEnumContainers
	return p
}

// defaultEnumContainers enumerates and reconciles a node's containers via the
// real Docker client. It is the production value of Poller.enumContainers.
// It also fires OOM-kill and unhealthy-healthcheck notifications via p.notify.
func (p *Poller) defaultEnumContainers(ctx context.Context, cl containerLister, nodeID string) ([]Delta, error) {
	infos, rawErr := enumDockerRaw(ctx, cl)
	if rawErr != nil {
		return nil, rawErr
	}
	p.checkContainerAlerts(ctx, nodeID, infos)
	cs := rawToDBContainers(infos, nodeID)
	return reconcileContainers(ctx, p.store, nodeID, cs)
}

// checkContainerAlerts fires OOM and unhealthy notifications for containers
// that are in a problematic state. The per-(webhook,trigger) rate window in the
// dispatcher prevents alert floods on persistent failures.
func (p *Poller) checkContainerAlerts(ctx context.Context, nodeID string, infos []containerInfo) {
	if p.notify == nil {
		return
	}
	for _, c := range infos {
		if c.State == "dead" {
			p.notify(ctx, triggerContainerOOM,
				"Container OOM-killed",
				fmt.Sprintf("Container %q on node %s was OOM-killed (state: dead)", c.Name, nodeID),
			)
		}
		if c.HealthStatus == "unhealthy" {
			p.notify(ctx, triggerContainerUnhealthy,
				"Container healthcheck unhealthy",
				fmt.Sprintf("Container %q on node %s healthcheck is unhealthy", c.Name, nodeID),
			)
		}
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

	// Docker enumeration runs purely off the Docker endpoint and is INDEPENDENT
	// of the SSH probe: a node whose SSH dial fails but whose Docker endpoint is
	// healthy still enumerates containers here and is marked reachable below.
	enum := p.enumContainers
	if enum == nil {
		enum = p.defaultEnumContainers
	}
	if caps.Docker && clients.Docker != nil {
		if d, err := enum(ctx, clients.Docker, n.ID); err == nil {
			reachable = true
			deltas = append(deltas, d...)
		} else {
			p.logger.Warn("inventory: docker enumeration failed", "node", n.ID, "error", err)
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

	// Reachability fallback: Docker/Proxmox enumeration doesn't cover SSH-only
	// nodes (nor a node whose Docker/Proxmox transport is down while SSH is up).
	// Probe SSH so those nodes are correctly "ok", and record why when they're
	// not — without this they sat "unreachable" with an empty last_error.
	// Map to a tri-state status. The fast enum path (Docker/Proxmox) only knows
	// "reachable"; the SSH-inclusive probe fallback can additionally report
	// "degraded" (API up, SSH down).
	status := "unreachable"
	var lastErr string
	if reachable {
		status = "ok"
	} else if p.reach != nil {
		switch st, le := p.reach(ctx, n); st {
		case "ok":
			status = "ok"
		case "degraded":
			status, lastErr = "degraded", le
		default:
			lastErr = le // status stays "unreachable"
		}
	}

	p.updateNodeStatus(ctx, n, status, lastErr)

	seq := p.seq.next(n.ID)
	for i := range deltas {
		deltas[i].Seq = seq
	}
	msg, err := json.Marshal(CycleMessage{NodeID: n.ID, Seq: seq, Deltas: deltas})
	if err == nil {
		p.hub.Broadcast("tree:"+n.ID, msg)
	}
}

func (p *Poller) updateNodeStatus(ctx context.Context, n db.Node, status, lastErr string) {
	// "ok" and "degraded" both count as reached (the node responded on some path).
	reached := status == "ok" || status == "degraded"
	newLastErr := lastErr
	if status == "ok" {
		newLastErr = "" // degraded keeps its SSH error as last_error
	}
	if reached {
		now := time.Now()
		n.LastSeen = &now
	}
	// Avoid a write storm on a persistently-down node, but still write if the
	// recorded error category changed (so last_error reflects the live reason).
	if n.Status == status && n.LastError == newLastErr && !reached {
		return
	}
	n.Status = status
	n.LastError = newLastErr
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
