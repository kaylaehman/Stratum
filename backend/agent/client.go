package agent

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/google/uuid"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	stratumv1 "github.com/kaylaehman/stratum/proto/gen/stratum/v1"
)

// streamBackoff is the reconnection schedule for WatchFiles streams.
// Production: base=1s, factor=2, cap=30s, full jitter.
var streamBackoff = defaultBackoff

// Client wraps a gRPC connection to one agent instance.
type Client struct {
	conn   *grpc.ClientConn
	rpc    stratumv1.AgentServiceClient
	nodeID string
	logger *slog.Logger
}

// Manager manages per-node agent connections. It is safe for concurrent use.
type Manager struct {
	store  db.Store
	logger *slog.Logger

	// TLS config shared across all agent connections (backend CA + client cert).
	tlsCfg *tls.Config

	mu      sync.Mutex
	clients map[string]*Client // nodeID → client
}

// NewManager creates a Manager. If tlsCfg is nil all agent-enabled nodes will
// fail to connect — callers must pass a valid *tls.Config for production use.
func NewManager(store db.Store, tlsCfg *tls.Config, logger *slog.Logger) *Manager {
	return &Manager{
		store:   store,
		logger:  logger,
		tlsCfg:  tlsCfg,
		clients: make(map[string]*Client),
	}
}

// Get returns a connected Client for nodeID, dialing on first use.
// Returns an error if the node lacks the agent capability or dialing fails.
func (m *Manager) Get(ctx context.Context, nodeID string) (*Client, error) {
	m.mu.Lock()
	if c, ok := m.clients[nodeID]; ok {
		m.mu.Unlock()
		return c, nil
	}
	m.mu.Unlock()

	node, err := m.store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if err := capabilities.Require(caps, capabilities.Agent); err != nil {
		return nil, fmt.Errorf("agent: %w", err)
	}

	addr := agentAddr(node)
	if addr == "" {
		return nil, fmt.Errorf("agent: cannot determine agent address for node %s", nodeID)
	}

	if m.tlsCfg == nil {
		return nil, fmt.Errorf("agent: no TLS config available; cannot connect to agent on node %s", nodeID)
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(m.tlsCfg)))
	if err != nil {
		return nil, fmt.Errorf("agent: dial %s: %w", addr, err)
	}

	c := &Client{
		conn:   conn,
		rpc:    stratumv1.NewAgentServiceClient(conn),
		nodeID: nodeID,
		logger: m.logger,
	}

	m.mu.Lock()
	if existing, ok := m.clients[nodeID]; ok {
		m.mu.Unlock()
		conn.Close()
		return existing, nil
	}
	m.clients[nodeID] = c
	m.mu.Unlock()
	return c, nil
}

// Invalidate closes and removes the cached client for nodeID.
func (m *Manager) Invalidate(nodeID string) {
	m.mu.Lock()
	c, ok := m.clients[nodeID]
	delete(m.clients, nodeID)
	m.mu.Unlock()
	if ok && c.conn != nil {
		_ = c.conn.Close()
	}
}

// Close shuts down all cached connections.
func (m *Manager) Close() {
	m.mu.Lock()
	all := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		all = append(all, c)
	}
	m.clients = make(map[string]*Client)
	m.mu.Unlock()
	for _, c := range all {
		if c.conn != nil {
			_ = c.conn.Close()
		}
	}
}

// InvalidateAll closes and removes every cached client. Used by certwatch when
// certificates are rotated so all subsequent dials use the new credentials.
func (m *Manager) InvalidateAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.clients))
	for id := range m.clients {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Invalidate(id)
	}
}

// DetectInit calls DetectInit on the agent and returns the raw response.
func (c *Client) DetectInit(ctx context.Context) (*stratumv1.DetectInitResponse, error) {
	return c.rpc.DetectInit(ctx, &stratumv1.DetectInitRequest{})
}

// StreamEvents opens a WatchFiles stream and delivers FileEvent values on the
// returned channel. The stream reconnects automatically on drop using
// exponential backoff with full jitter (base=1s, cap=30s). The caller must
// cancel ctx to stop streaming.
func (c *Client) StreamEvents(ctx context.Context, paths []string, recursive bool) <-chan *stratumv1.WatchFilesResponse {
	ch := make(chan *stratumv1.WatchFilesResponse, 256)
	go func() {
		defer close(ch)
		attempt := 0
		for {
			if err := c.streamOnce(ctx, paths, recursive, ch); err != nil {
				if ctx.Err() != nil {
					return
				}
				delay := streamBackoff.next(attempt)
				attempt++
				c.logger.Warn("agent: stream error; will reconnect",
					"node", c.nodeID, "error", err, "attempt", attempt, "delay", delay)
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
				}
			} else {
				// Clean exit (EOF or ctx cancelled): reset backoff counter.
				attempt = 0
			}
		}
	}()
	return ch
}

func (c *Client) streamOnce(ctx context.Context, paths []string, recursive bool, ch chan<- *stratumv1.WatchFilesResponse) error {
	stream, err := c.rpc.WatchFiles(ctx, &stratumv1.WatchFilesRequest{
		Paths:     paths,
		Recursive: recursive,
	})
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	for {
		ev, err := stream.Recv()
		if err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case ch <- ev:
		default:
			c.logger.Warn("agent: event channel full; dropping", "node", c.nodeID, "path", ev.GetPath())
		}
	}
}

// agentAddr derives the gRPC address for the agent from node metadata.
// Convention: host:7750 (default agent port).
func agentAddr(node db.Node) string {
	if node.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s:7750", node.Host)
}

// PersistEvents reads from ch and inserts each event into the FileEvent store.
// It logs warnings on persistence failures but never stops. Call from a
// goroutine; returns when ch is closed.
func PersistEvents(ctx context.Context, nodeID string, ch <-chan *stratumv1.WatchFilesResponse, store db.Store, logger *slog.Logger) {
	for ev := range ch {
		fe := db.FileEvent{
			ID:         uuid.NewString(),
			NodeID:     nodeID,
			Path:       ev.GetPath(),
			EventType:  protoEventTypeToDB(ev.GetEventType()),
			DetectedAt: ev.GetTimestamp().AsTime(),
		}
		if err := store.InsertFileEvent(ctx, fe); err != nil {
			logger.Warn("agent: persist file event", "node", nodeID, "path", fe.Path, "error", err)
		}
	}
}

// protoEventTypeToDB maps the proto FileEventType to the DB string used by
// the existing filewatch.Service.
func protoEventTypeToDB(t stratumv1.FileEventType) string {
	switch t {
	case stratumv1.FileEventType_FILE_EVENT_TYPE_CREATE:
		return "create"
	case stratumv1.FileEventType_FILE_EVENT_TYPE_MODIFY:
		return "modified"
	case stratumv1.FileEventType_FILE_EVENT_TYPE_DELETE:
		return "delete"
	case stratumv1.FileEventType_FILE_EVENT_TYPE_RENAME:
		return "rename"
	case stratumv1.FileEventType_FILE_EVENT_TYPE_ATTRIB:
		return "attrib"
	default:
		return "modified"
	}
}
