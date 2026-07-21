package logtail

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/KAE-Labs/stratum/backend/docker"
	"github.com/KAE-Labs/stratum/backend/hub"
)

// ErrUnauthorized is returned by Subscribe when the Authorizer denies access.
var ErrUnauthorized = errors.New("logtail: unauthorized")

// Authorizer reports whether a user may read a node's containers.
type Authorizer func(ctx context.Context, userID, nodeID string) (bool, error)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// HubGrants is the subset of hub.Hub the manager needs.
type HubGrants interface {
	Subscribe(clientID hub.ClientID, topic string)
	Broadcast(topic string, payload []byte)
}

// startTailerFunc is the pluggable tailer-start hook; swapped out in tests.
type startTailerFunc func(ctx context.Context, cancel context.CancelFunc, containerID string, client tailClient, publish func(LogLine))

// tailerEntry tracks a running tailer and its refcount.
type tailerEntry struct {
	cancel    context.CancelFunc
	refcount  int
	listeners []func(LogLine)
}

// clientSub records one client's subscription to a docker container.
type clientSub struct {
	dockerID string
	nodeID   string
}

// Manager runs refcounted per-container tailers and fans out to the hub.
type Manager struct {
	mu       sync.Mutex
	provider ClientProvider
	hub      HubGrants
	authz    Authorizer
	baseCtx  context.Context

	tailers map[string]*tailerEntry      // key: dockerID
	clients map[hub.ClientID][]clientSub // key: clientID -> subs

	// startTailer is called when refcount goes 0->1.
	// Overridden in tests to avoid starting a real goroutine.
	startTailer startTailerFunc
}

// NewManager creates a Manager. The manager inherits the lifecycle of baseCtx.
func NewManager(provider ClientProvider, h HubGrants, authz Authorizer) *Manager {
	m := &Manager{
		provider: provider,
		hub:      h,
		authz:    authz,
		baseCtx:  context.Background(),
		tailers:  make(map[string]*tailerEntry),
		clients:  make(map[hub.ClientID][]clientSub),
	}
	m.startTailer = m.defaultStartTailer
	return m
}

// topic returns the hub topic string for a container's log stream.
func topic(dockerID string) string {
	return "logs:" + dockerID
}

// Subscribe authorises the caller, increments the refcount for dockerID,
// starts a tailer when refcount goes 0->1, and grants the hub client the topic.
func (m *Manager) Subscribe(
	ctx context.Context,
	userID, nodeID, containerID, dockerID string,
	clientID hub.ClientID,
) error {
	ok, err := m.authz(ctx, userID, nodeID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUnauthorized
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.tailers[dockerID]
	if !exists {
		tailerCtx, cancel := context.WithCancel(m.baseCtx)
		// Create the entry fully-formed (refcount 1) and store it BEFORE
		// launching the tailer goroutine, so any concurrent CancelContainer sees
		// a live entry whose cancel also stops this tailer (no phantom entry).
		entry = &tailerEntry{cancel: cancel, refcount: 1}
		m.tailers[dockerID] = entry
		go func() {
			client, clientErr := m.provider(tailerCtx, nodeID)
			if clientErr != nil {
				// Provider unavailable — emit synthetic error line then exit.
				m.publishLine(dockerID, LogLine{
					ContainerID: dockerID,
					Stream:      "stdout",
					Text:        "— tailer error: could not connect to node —",
				})
				return
			}
			publish := func(ll LogLine) { m.publishLine(dockerID, ll) }
			m.startTailer(tailerCtx, cancel, dockerID, client, publish)
		}()
	} else {
		entry.refcount++
	}

	m.hub.Subscribe(clientID, topic(dockerID))
	m.clients[clientID] = append(m.clients[clientID], clientSub{dockerID: dockerID, nodeID: nodeID})
	_ = containerID // stored for context; routing uses dockerID
	return nil
}

// Unsubscribe decrements the refcount for dockerID. When it reaches zero the
// tailer is cancelled.
func (m *Manager) Unsubscribe(dockerID string, clientID hub.ClientID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decref(dockerID, clientID)
}

// ReleaseClient decrements all subscriptions belonging to clientID (called on
// WebSocket disconnect).
func (m *Manager) ReleaseClient(clientID hub.ClientID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	subs := m.clients[clientID]
	delete(m.clients, clientID)
	for _, s := range subs {
		m.decrefLocked(s.dockerID)
	}
}

// CancelContainer force-stops a tailer (called on a container removed event).
func (m *Manager) CancelContainer(dockerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.tailers[dockerID]; ok {
		entry.cancel()
		delete(m.tailers, dockerID)
	}
}

// AddListener registers a server-side callback for lines on dockerID.
// The callback is invoked synchronously in the tailer's publish path
// (Feature 26 / notification seam). Returns a no-op remove func.
func (m *Manager) AddListener(dockerID string, fn func(LogLine)) func() {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.tailers[dockerID]
	if !ok {
		// Tailer not yet started; will be attached on first Subscribe.
		// For simplicity, listeners added before a tailer starts are lost.
		return func() {}
	}
	entry.listeners = append(entry.listeners, fn)
	idx := len(entry.listeners) - 1
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if e, ok := m.tailers[dockerID]; ok && idx < len(e.listeners) {
			e.listeners[idx] = nil // nil = removed
		}
	}
}

// publishLine broadcasts a log line to hub subscribers and calls any
// registered server-side listeners. Safe to call from outside the lock.
func (m *Manager) publishLine(dockerID string, ll LogLine) {
	payload, _ := json.Marshal(ll)
	m.hub.Broadcast(topic(dockerID), payload)

	m.mu.Lock()
	entry := m.tailers[dockerID]
	var listeners []func(LogLine)
	if entry != nil {
		listeners = make([]func(LogLine), len(entry.listeners))
		copy(listeners, entry.listeners)
	}
	m.mu.Unlock()

	for _, fn := range listeners {
		if fn != nil {
			fn(ll)
		}
	}
}

// decref decrements refcount for dockerID and removes clientID's record.
// Must be called with m.mu held only for the client-map portion; uses
// decrefLocked for the tailer portion.
func (m *Manager) decref(dockerID string, clientID hub.ClientID) {
	// Remove from client map.
	subs := m.clients[clientID]
	for i, s := range subs {
		if s.dockerID == dockerID {
			m.clients[clientID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(m.clients[clientID]) == 0 {
		delete(m.clients, clientID)
	}
	m.decrefLocked(dockerID)
}

// decrefLocked decrements the tailer refcount. Cancels and removes the entry
// when it reaches zero. Must be called with m.mu held.
func (m *Manager) decrefLocked(dockerID string) {
	entry, ok := m.tailers[dockerID]
	if !ok {
		return
	}
	entry.refcount--
	if entry.refcount <= 0 {
		entry.cancel()
		delete(m.tailers, dockerID)
	}
}

// defaultStartTailer is the production tailer goroutine launcher.
func (m *Manager) defaultStartTailer(
	ctx context.Context,
	_ context.CancelFunc,
	containerID string,
	client tailClient,
	publish func(LogLine),
) {
	t := &tailer{
		client:      client,
		containerID: containerID,
		publish:     publish,
	}
	t.run(ctx, "100")
}
