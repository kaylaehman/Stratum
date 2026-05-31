package uptime

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/webhooks"
)

// NotifyFunc matches the webhook dispatcher's Notify signature.
type NotifyFunc func(ctx context.Context, trigger string, msg webhooks.Message)

// DefaultConsecutiveFailuresRequired is the number of consecutive failed probes
// required before a monitor is declared DOWN and a webhook notification fires.
// A single transient timeout or brief TLS-handshake delay must not page.
const DefaultConsecutiveFailuresRequired = 2

// DefaultTimeoutMs is the per-monitor HTTP/TCP timeout used when the monitor
// record carries a zero or unset value. 10 s is generous enough for TLS
// hand-shake on a slow homelab host without being so long that failures pile up.
const DefaultTimeoutMs = 10_000

// monitorState holds the in-memory consecutive-failure counter and last
// declared status for one monitor. It lives only in the Service and is
// intentionally not persisted — a restart resets the counter, which is safe
// (worst case: one extra transient failure is needed to re-trigger an alert).
type monitorState struct {
	lastDeclaredStatus CheckStatus
	consecutiveFails   int
}

// Service manages the periodic checker loop for all enabled monitors.
type Service struct {
	store   Store
	checker Checker
	notify  NotifyFunc
	logger  *slog.Logger

	// consecutiveFailuresRequired is the threshold before DOWN is declared.
	// Defaults to DefaultConsecutiveFailuresRequired; overridable in tests.
	consecutiveFailuresRequired int

	mu     sync.Mutex
	states map[string]*monitorState // per monitor ID
}

// New creates a Service with the production NetChecker.
func New(store Store, logger *slog.Logger) *Service {
	return &Service{
		store:                       store,
		checker:                     &NetChecker{},
		notify:                      func(_ context.Context, _ string, _ webhooks.Message) {},
		logger:                      logger,
		consecutiveFailuresRequired: DefaultConsecutiveFailuresRequired,
		states:                      map[string]*monitorState{},
	}
}

// SetNotify wires the webhook notification function.
func (s *Service) SetNotify(fn NotifyFunc) { s.notify = fn }

// SetChecker replaces the checker (test injection point).
func (s *Service) SetChecker(c Checker) { s.checker = c }

// Run starts the ticker loop; it blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) {
	const tickInterval = 10 * time.Second
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	lastRun := map[string]time.Time{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			monitors, err := s.store.ListUptimeMonitors(ctx)
			if err != nil {
				s.logger.Warn("uptime: list monitors", "error", err)
				continue
			}
			for _, dbm := range monitors {
				if !dbm.Enabled {
					continue
				}
				interval := time.Duration(dbm.IntervalSeconds) * time.Second
				if time.Since(lastRun[dbm.ID]) < interval {
					continue
				}
				lastRun[dbm.ID] = time.Now()
				go s.runCheck(ctx, dbToMonitor(dbm))
			}
		}
	}
}

// runCheck executes one check for m, persists the result, and fires a
// webhook only after N consecutive failures (consecutive-failure state machine).
func (s *Service) runCheck(ctx context.Context, m Monitor) {
	// Use the monitor's configured timeout as the outer deadline so the context
	// covers connect + TLS + headers with room to spare.
	timeoutMs := m.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = DefaultTimeoutMs
	}
	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond+2*time.Second)
	defer cancel()

	result := s.checker.Check(checkCtx, m)
	result.ID = uuid.NewString()

	if err := s.store.InsertUptimeResult(ctx, resultToDb(result)); err != nil {
		s.logger.Warn("uptime: insert result", "monitor_id", m.ID, "error", err)
	}

	s.mu.Lock()
	st := s.states[m.ID]
	if st == nil {
		st = &monitorState{}
		s.states[m.ID] = st
	}
	shouldNotify := s.applyResult(st, result.Status)
	s.mu.Unlock()

	if shouldNotify {
		s.notify(ctx, TriggerUptimeDown, webhooks.Message{
			Title: "Monitor down: " + m.Name,
			Text:  "Target " + m.Target + " is unreachable. Error: " + result.Error,
		})
	}
}

// applyResult updates st in-place and returns true if a DOWN notification
// should fire. Must be called with s.mu held.
//
// Rules:
//   - Each failure increments consecutiveFails.
//   - DOWN is declared (and notified once) only when consecutiveFails reaches
//     the required threshold AND the monitor was not already declared DOWN.
//   - Any success resets consecutiveFails and clears the DOWN declaration so
//     a future failure streak can re-alert.
func (s *Service) applyResult(st *monitorState, status CheckStatus) (notify bool) {
	if status == StatusDown || status == StatusDegraded {
		st.consecutiveFails++
		if st.consecutiveFails >= s.consecutiveFailuresRequired &&
			st.lastDeclaredStatus != StatusDown {
			st.lastDeclaredStatus = StatusDown
			return true
		}
		return false
	}
	// Successful probe: reset failure streak and clear declared-down state.
	st.consecutiveFails = 0
	st.lastDeclaredStatus = status
	return false
}

// RunPrune periodically removes results older than maxAge.
func (s *Service) RunPrune(ctx context.Context, maxAge time.Duration) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-maxAge)
			n, err := s.store.PruneUptimeResultsBefore(ctx, cutoff)
			if err != nil {
				s.logger.Warn("uptime: prune results", "error", err)
			} else if n > 0 {
				s.logger.Info("uptime: pruned old results", "count", n)
			}
		}
	}
}

// dbToMonitor converts a db.UptimeMonitor to the local Monitor type.
func dbToMonitor(m db.UptimeMonitor) Monitor {
	return Monitor{
		ID:              m.ID,
		Name:            m.Name,
		Type:            CheckType(m.Type),
		Target:          m.Target,
		IntervalSeconds: m.IntervalSeconds,
		TimeoutMs:       m.TimeoutMs,
		Expected:        m.Expected,
		Enabled:         m.Enabled,
		NodeID:          m.NodeID,
	}
}

// resultToDb converts a local Result to db.UptimeResult.
func resultToDb(r Result) db.UptimeResult {
	return db.UptimeResult{
		ID:             r.ID,
		MonitorID:      r.MonitorID,
		CheckedAt:      r.CheckedAt,
		Status:         string(r.Status),
		ResponseTimeMs: r.ResponseTimeMs,
		Error:          r.Error,
	}
}
