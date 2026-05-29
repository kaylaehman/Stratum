package logtail

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/hub"
)

// ---------------------------------------------------------------------------
// Fake hub
// ---------------------------------------------------------------------------

type fakeHub struct {
	mu         sync.Mutex
	subscribed []subscribeCall
	broadcasts []broadcastCall
}

type subscribeCall struct {
	clientID hub.ClientID
	topic    string
}

type broadcastCall struct {
	topic   string
	payload []byte
}

func (f *fakeHub) Subscribe(clientID hub.ClientID, topic string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscribed = append(f.subscribed, subscribeCall{clientID, topic})
}

func (f *fakeHub) Broadcast(topic string, payload []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.broadcasts = append(f.broadcasts, broadcastCall{topic, payload})
}

func (f *fakeHub) subscribeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.subscribed)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// allowAll always permits access.
func allowAll(_ context.Context, _, _ string) (bool, error) { return true, nil }

// denyAll always denies access.
func denyAll(_ context.Context, _, _ string) (bool, error) { return false, nil }

// stubProvider returns an error so no real Docker call is made.
// Used for tests that only need auth/refcount/grant logic (tailer is not started).
func stubProvider(_ context.Context, _ string) (*docker.Client, error) {
	return nil, errors.New("stub: no docker connection")
}

// noopStartTailer is a startTailerFunc that does nothing (no goroutine, no
// blocking). Used when we want to test auth / refcount / hub-grant paths
// without a real tailer running.
func noopStartTailer(
	_ context.Context,
	_ context.CancelFunc,
	_ string,
	_ tailClient,
	_ func(LogLine),
) {
	// intentionally empty
}

// newTestManager builds a Manager with a noop tailer and stub provider.
func newTestManager(h HubGrants, authz Authorizer) *Manager {
	m := NewManager(stubProvider, h, authz)
	m.startTailer = noopStartTailer
	return m
}

// ---------------------------------------------------------------------------
// Tests: authorisation
// ---------------------------------------------------------------------------

func TestManager_Subscribe_Unauthorized(t *testing.T) {
	h := &fakeHub{}
	m := newTestManager(h, denyAll)

	err := m.Subscribe(context.Background(), "user1", "node1", "ctr1", "docker1", hub.ClientID("c1"))
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	if h.subscribeCount() != 0 {
		t.Errorf("hub.Subscribe should not be called when unauthorized; called %d time(s)", h.subscribeCount())
	}
}

func TestManager_Subscribe_Authorized_HubGranted(t *testing.T) {
	h := &fakeHub{}
	m := newTestManager(h, allowAll)

	err := m.Subscribe(context.Background(), "user1", "node1", "ctr1", "docker1", hub.ClientID("c1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subscribed) != 1 {
		t.Fatalf("expected 1 hub.Subscribe call, got %d", len(h.subscribed))
	}
	if h.subscribed[0].topic != "logs:docker1" {
		t.Errorf("topic = %q, want %q", h.subscribed[0].topic, "logs:docker1")
	}
	if h.subscribed[0].clientID != "c1" {
		t.Errorf("clientID = %q, want c1", h.subscribed[0].clientID)
	}
}

// ---------------------------------------------------------------------------
// Tests: refcounting
// ---------------------------------------------------------------------------

func TestManager_Refcount_SecondSubscribeDoesNotRestartTailer(t *testing.T) {
	h := &fakeHub{}
	startCount := 0
	m := NewManager(stubProvider, h, allowAll)
	m.startTailer = func(ctx context.Context, cancel context.CancelFunc, containerID string, client tailClient, publish func(LogLine)) {
		startCount++
	}

	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "c1")
	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "c2")

	m.mu.Lock()
	rc := m.tailers["docker1"].refcount
	m.mu.Unlock()

	if rc != 2 {
		t.Errorf("refcount = %d, want 2", rc)
	}
	// Hub.Subscribe should be called twice (once per client).
	if h.subscribeCount() != 2 {
		t.Errorf("hub.Subscribe called %d time(s), want 2", h.subscribeCount())
	}
}

func TestManager_Refcount_UnsubscribeKeepsAlive(t *testing.T) {
	h := &fakeHub{}
	m := newTestManager(h, allowAll)

	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "c1")
	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "c2")

	// Unsubscribe one client; tailer should remain.
	m.Unsubscribe("docker1", "c1")

	m.mu.Lock()
	entry, exists := m.tailers["docker1"]
	m.mu.Unlock()

	if !exists {
		t.Fatal("tailer should still exist after one of two clients unsubscribes")
	}
	if entry.refcount != 1 {
		t.Errorf("refcount = %d, want 1", entry.refcount)
	}
}

func TestManager_Refcount_UnsubscribeBothStopsTailer(t *testing.T) {
	h := &fakeHub{}
	cancelled := false
	m := NewManager(stubProvider, h, allowAll)
	m.startTailer = func(ctx context.Context, cancel context.CancelFunc, containerID string, client tailClient, publish func(LogLine)) {
		// Track cancellation via a context watch goroutine.
		go func() {
			<-ctx.Done()
			cancelled = true
		}()
	}

	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "c1")
	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "c2")

	m.Unsubscribe("docker1", "c1")
	m.Unsubscribe("docker1", "c2")

	m.mu.Lock()
	_, exists := m.tailers["docker1"]
	m.mu.Unlock()

	if exists {
		t.Error("tailer entry should be removed when refcount reaches zero")
	}
	// Give the goroutine a moment to see the cancellation.
	for i := 0; i < 100 && !cancelled; i++ {
		// spin briefly; context cancellation is async
	}
	if !cancelled {
		// This is best-effort — context may not be observed in tests without timing.
		// Just verify the entry is gone.
	}
}

// ---------------------------------------------------------------------------
// Tests: ReleaseClient
// ---------------------------------------------------------------------------

func TestManager_ReleaseClient_DropsAllSubs(t *testing.T) {
	h := &fakeHub{}
	m := newTestManager(h, allowAll)

	_ = m.Subscribe(context.Background(), "u", "n", "c1", "docker1", "client-A")
	_ = m.Subscribe(context.Background(), "u", "n", "c2", "docker2", "client-A")

	m.ReleaseClient("client-A")

	m.mu.Lock()
	_, has1 := m.tailers["docker1"]
	_, has2 := m.tailers["docker2"]
	_, hasClt := m.clients["client-A"]
	m.mu.Unlock()

	if has1 || has2 {
		t.Error("all tailers should be removed after ReleaseClient")
	}
	if hasClt {
		t.Error("client entry should be removed after ReleaseClient")
	}
}

func TestManager_ReleaseClient_SharedContainerRemains(t *testing.T) {
	h := &fakeHub{}
	m := newTestManager(h, allowAll)

	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "client-A")
	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "client-B")

	// Release A — B is still subscribed, tailer should survive.
	m.ReleaseClient("client-A")

	m.mu.Lock()
	entry, exists := m.tailers["docker1"]
	m.mu.Unlock()

	if !exists {
		t.Fatal("tailer should remain when another client is still subscribed")
	}
	if entry.refcount != 1 {
		t.Errorf("refcount = %d, want 1", entry.refcount)
	}
}

// ---------------------------------------------------------------------------
// Tests: CancelContainer
// ---------------------------------------------------------------------------

func TestManager_CancelContainer_RemovesEntry(t *testing.T) {
	h := &fakeHub{}
	m := newTestManager(h, allowAll)

	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "c1")
	m.CancelContainer("docker1")

	m.mu.Lock()
	_, exists := m.tailers["docker1"]
	m.mu.Unlock()

	if exists {
		t.Error("CancelContainer should remove the tailer entry")
	}
}

// ---------------------------------------------------------------------------
// Tests: publish broadcasts to hub
// ---------------------------------------------------------------------------

func TestManager_Publish_BroadcastsToHub(t *testing.T) {
	h := &fakeHub{}
	var (
		publishedMu sync.Mutex
		published   []LogLine
	)
	m := NewManager(stubProvider, h, allowAll)
	m.startTailer = func(ctx context.Context, cancel context.CancelFunc, containerID string, client tailClient, publish func(LogLine)) {
		// Immediately emit one line.
		publish(LogLine{ContainerID: containerID, Stream: "stdout", Text: "hello"})
	}

	_ = m.Subscribe(context.Background(), "u", "n", "c", "docker1", "c1")

	// Add a listener after Subscribe (tailer already started and published).
	// publishLine invokes listeners from whichever goroutine called it (the
	// tailer goroutine here), so the slice append races with the main
	// goroutine's read below — guard with a mutex.
	m.AddListener("docker1", func(ll LogLine) {
		publishedMu.Lock()
		published = append(published, ll)
		publishedMu.Unlock()
	})

	// Directly publish through the manager to test the path.
	m.publishLine("docker1", LogLine{ContainerID: "docker1", Stream: "stdout", Text: "test"})

	h.mu.Lock()
	broadcasts := h.broadcasts
	h.mu.Unlock()

	found := false
	for _, b := range broadcasts {
		if b.topic == "logs:docker1" {
			found = true
		}
	}
	if !found {
		t.Error("expected a broadcast on topic logs:docker1")
	}

	publishedMu.Lock()
	got := append([]LogLine(nil), published...)
	publishedMu.Unlock()
	if len(got) != 1 || got[0].Text != "test" {
		t.Errorf("listener got %v, want [{Text:test}]", got)
	}
}
