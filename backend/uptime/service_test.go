package uptime

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/webhooks"
)

func newNopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- State machine unit tests ---

// TestApplyResult_SingleFailureNoAlert verifies that one failed probe does NOT
// trigger a DOWN notification — the core fix for the false-positive alert bug.
func TestApplyResult_SingleFailureNoAlert(t *testing.T) {
	svc := newTestService()
	st := &monitorState{}
	if got := svc.applyResult(st, StatusDown); got {
		t.Error("single failure should NOT notify (got notify=true)")
	}
	if st.lastDeclaredStatus == StatusDown {
		t.Error("single failure should not declare status as DOWN yet")
	}
	if st.consecutiveFails != 1 {
		t.Errorf("consecutiveFails = %d, want 1", st.consecutiveFails)
	}
}

// TestApplyResult_NConsecutiveFailuresAlert verifies that exactly N consecutive
// failures triggers ONE DOWN notification.
func TestApplyResult_NConsecutiveFailuresAlert(t *testing.T) {
	svc := newTestService()
	st := &monitorState{}

	for i := 0; i < DefaultConsecutiveFailuresRequired-1; i++ {
		if got := svc.applyResult(st, StatusDown); got {
			t.Errorf("failure %d of %d: should NOT notify yet", i+1, DefaultConsecutiveFailuresRequired)
		}
	}
	// N-th failure: should notify now.
	if got := svc.applyResult(st, StatusDown); !got {
		t.Errorf("failure %d: should notify on reaching threshold", DefaultConsecutiveFailuresRequired)
	}
	if st.lastDeclaredStatus != StatusDown {
		t.Errorf("lastDeclaredStatus = %s, want down", st.lastDeclaredStatus)
	}
}

// TestApplyResult_NoDoubleAlert verifies that being already DOWN does not re-fire
// on every subsequent failed probe.
func TestApplyResult_NoDoubleAlert(t *testing.T) {
	svc := newTestService()
	st := &monitorState{lastDeclaredStatus: StatusDown, consecutiveFails: DefaultConsecutiveFailuresRequired}
	for i := 0; i < 5; i++ {
		if got := svc.applyResult(st, StatusDown); got {
			t.Errorf("probe %d: should NOT re-notify when already declared DOWN", i)
		}
	}
}

// TestApplyResult_RecoveryAfterDown verifies that a success after DOWN clears
// the state so a fresh failure streak can re-alert.
func TestApplyResult_RecoveryAfterDown(t *testing.T) {
	svc := newTestService()
	st := &monitorState{lastDeclaredStatus: StatusDown, consecutiveFails: DefaultConsecutiveFailuresRequired}

	// Recovery probe.
	if got := svc.applyResult(st, StatusUp); got {
		t.Error("recovery should not notify")
	}
	if st.consecutiveFails != 0 {
		t.Errorf("consecutiveFails after recovery = %d, want 0", st.consecutiveFails)
	}
	if st.lastDeclaredStatus == StatusDown {
		t.Errorf("lastDeclaredStatus should be cleared after recovery, got %s", st.lastDeclaredStatus)
	}

	// New failure streak after recovery should re-alert.
	for i := 0; i < DefaultConsecutiveFailuresRequired-1; i++ {
		svc.applyResult(st, StatusDown)
	}
	if got := svc.applyResult(st, StatusDown); !got {
		t.Error("should re-notify after recovery + new failure streak")
	}
}

// TestApplyResult_DegradedCountsAsFailure verifies that StatusDegraded
// also increments the failure counter (slow response is still a problem).
func TestApplyResult_DegradedCountsAsFailure(t *testing.T) {
	svc := newTestService()
	st := &monitorState{}
	for i := 0; i < DefaultConsecutiveFailuresRequired-1; i++ {
		svc.applyResult(st, StatusDegraded)
	}
	if got := svc.applyResult(st, StatusDegraded); !got {
		t.Error("N degraded probes should trigger DOWN notification")
	}
}

// --- Integration-style test: runCheck uses mock checker + store ---

// mockChecker returns a fixed sequence of statuses.
type mockChecker struct {
	mu       sync.Mutex
	statuses []CheckStatus
	pos      int
}

func (m *mockChecker) Check(_ context.Context, mon Monitor) Result {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.statuses[m.pos%len(m.statuses)]
	m.pos++
	return Result{MonitorID: mon.ID, CheckedAt: time.Now(), Status: s}
}

// memStore is an in-memory stub satisfying the Store interface.
type memStore struct {
	mu      sync.Mutex
	results []db.UptimeResult
}

func (ms *memStore) CreateUptimeMonitor(_ context.Context, _ db.UptimeMonitor) error { return nil }
func (ms *memStore) GetUptimeMonitor(_ context.Context, _ string) (db.UptimeMonitor, error) {
	return db.UptimeMonitor{}, db.ErrNotFound
}
func (ms *memStore) ListUptimeMonitors(_ context.Context) ([]db.UptimeMonitor, error) {
	return nil, nil
}
func (ms *memStore) UpdateUptimeMonitor(_ context.Context, _ db.UptimeMonitor) error { return nil }
func (ms *memStore) DeleteUptimeMonitor(_ context.Context, _ string) error           { return nil }
func (ms *memStore) InsertUptimeResult(_ context.Context, r db.UptimeResult) error {
	ms.mu.Lock()
	ms.results = append(ms.results, r)
	ms.mu.Unlock()
	return nil
}
func (ms *memStore) ListUptimeResults(_ context.Context, _ string, _, _ time.Time) ([]db.UptimeResult, error) {
	return nil, nil
}
func (ms *memStore) LatestUptimeResult(_ context.Context, _ string) (db.UptimeResult, error) {
	return db.UptimeResult{}, db.ErrNotFound
}
func (ms *memStore) PruneUptimeResultsBefore(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

// newTestService builds a Service with threshold=2 and no logger noise.
func newTestService() *Service {
	s := New(&memStore{}, nil)
	s.logger = newNopLogger()
	return s
}

// TestRunCheck_NotifiesAfterThreshold runs runCheck in sequence using a mock
// checker that returns DOWN, then verifies notifications fire only at threshold.
func TestRunCheck_NotifiesAfterThreshold(t *testing.T) {
	store := &memStore{}
	svc := &Service{
		store:                       store,
		checker:                     &mockChecker{statuses: []CheckStatus{StatusDown, StatusDown, StatusDown}},
		consecutiveFailuresRequired: 2,
		states:                      map[string]*monitorState{},
		logger:                      newNopLogger(),
	}

	var notifyCount int
	svc.notify = func(_ context.Context, _ string, _ webhooks.Message) { notifyCount++ }

	mon := Monitor{ID: "m1", Name: "test", Type: CheckHTTP, Target: "http://x", TimeoutMs: 500}

	svc.runCheck(context.Background(), mon) // fail 1 → no notify
	if notifyCount != 0 {
		t.Errorf("after 1 failure: notifyCount = %d, want 0", notifyCount)
	}

	svc.runCheck(context.Background(), mon) // fail 2 → notify
	if notifyCount != 1 {
		t.Errorf("after 2 failures: notifyCount = %d, want 1", notifyCount)
	}

	svc.runCheck(context.Background(), mon) // fail 3 → already DOWN, no double-notify
	if notifyCount != 1 {
		t.Errorf("after 3 failures: notifyCount = %d, want 1 (no double-notify)", notifyCount)
	}
}

// TestRunCheck_RecoveryThenAlert verifies the full UP→DOWN→UP→DOWN cycle fires
// at most one notification per downtime period.
func TestRunCheck_RecoveryThenAlert(t *testing.T) {
	store := &memStore{}
	seq := []CheckStatus{StatusDown, StatusDown, StatusUp, StatusDown, StatusDown}
	svc := &Service{
		store:                       store,
		checker:                     &mockChecker{statuses: seq},
		consecutiveFailuresRequired: 2,
		states:                      map[string]*monitorState{},
		logger:                      newNopLogger(),
	}

	var notifyCount int
	svc.notify = func(_ context.Context, _ string, _ webhooks.Message) { notifyCount++ }

	mon := Monitor{ID: "m2", Name: "test2", Type: CheckHTTP, Target: "http://x", TimeoutMs: 500}
	for range seq {
		svc.runCheck(context.Background(), mon)
	}
	// One alert for the first downtime, one for the second (after recovery).
	if notifyCount != 2 {
		t.Errorf("notifyCount = %d, want 2 (one per downtime period)", notifyCount)
	}
}
