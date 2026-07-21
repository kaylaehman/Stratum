package agent

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync"
	"time"

	"github.com/KAE-Labs/stratum/backend/capabilities"
	"github.com/KAE-Labs/stratum/backend/db"
)

// pollInterval is how often the orchestrator re-scans the node list to start
// streams for newly-added agent-capable nodes.
const pollInterval = 30 * time.Second

// Orchestrator starts and maintains WatchFiles streams for all agent-capable
// nodes, delivering events into the shared FileEvent store.
type Orchestrator struct {
	store   db.Store
	mgr     *Manager
	tlsCfg  *tls.Config
	logger  *slog.Logger

	mu      sync.Mutex
	running map[string]context.CancelFunc // nodeID → cancel
}

// NewOrchestrator wires the store, connection manager, TLS config, and logger.
func NewOrchestrator(store db.Store, mgr *Manager, tlsCfg *tls.Config, logger *slog.Logger) *Orchestrator {
	return &Orchestrator{
		store:   store,
		mgr:     mgr,
		tlsCfg:  tlsCfg,
		logger:  logger,
		running: make(map[string]context.CancelFunc),
	}
}

// Run polls the node list and starts streams for agent-capable nodes that do
// not yet have an active stream. It blocks until ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context) {
	if o.tlsCfg == nil {
		o.logger.Info("agent orchestrator: no TLS config; agent streaming disabled")
		return
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	o.reconcile(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.reconcile(ctx)
		}
	}
}

func (o *Orchestrator) reconcile(ctx context.Context) {
	nodes, err := o.store.ListNodes(ctx)
	if err != nil {
		o.logger.Warn("agent orchestrator: list nodes", "error", err)
		return
	}

	for _, node := range nodes {
		caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
		if !caps.Agent {
			continue
		}
		o.ensureStream(ctx, node)
	}
}

func (o *Orchestrator) ensureStream(ctx context.Context, node db.Node) {
	o.mu.Lock()
	if _, ok := o.running[node.ID]; ok {
		o.mu.Unlock()
		return
	}
	streamCtx, cancel := context.WithCancel(ctx)
	o.running[node.ID] = cancel
	o.mu.Unlock()

	watches, err := o.store.ListFileWatchesByNode(ctx, node.ID)
	if err != nil {
		o.logger.Warn("agent orchestrator: list watches", "node", node.ID, "error", err)
		o.mu.Lock()
		delete(o.running, node.ID)
		o.mu.Unlock()
		return
	}

	paths := make([]string, 0, len(watches))
	recursive := false
	for _, w := range watches {
		paths = append(paths, w.Path)
		if w.Recursive {
			recursive = true
		}
	}

	if len(paths) == 0 {
		// No watches configured yet; release the slot so we retry after next poll.
		o.mu.Lock()
		delete(o.running, node.ID)
		o.mu.Unlock()
		cancel()
		return
	}

	go func() {
		defer func() {
			o.mu.Lock()
			delete(o.running, node.ID)
			o.mu.Unlock()
			cancel()
		}()

		client, err := o.mgr.Get(streamCtx, node.ID)
		if err != nil {
			o.logger.Warn("agent orchestrator: get client", "node", node.ID, "error", err)
			return
		}

		ch := client.StreamEvents(streamCtx, paths, recursive)
		o.logger.Info("agent orchestrator: streaming file events", "node", node.ID, "paths", paths)
		PersistEvents(streamCtx, node.ID, ch, o.store, o.logger)
		o.logger.Info("agent orchestrator: stream ended", "node", node.ID)
	}()
}
