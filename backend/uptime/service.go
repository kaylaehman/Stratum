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

// Service manages the periodic checker loop for all enabled monitors.
type Service struct {
	store   Store
	checker Checker
	notify  NotifyFunc
	logger  *slog.Logger

	mu         sync.Mutex
	lastStatus map[string]CheckStatus // per monitor ID
}

// New creates a Service with the production NetChecker.
func New(store Store, logger *slog.Logger) *Service {
	return &Service{
		store:      store,
		checker:    &NetChecker{},
		notify:     func(_ context.Context, _ string, _ webhooks.Message) {},
		logger:     logger,
		lastStatus: map[string]CheckStatus{},
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
// webhook on UP→DOWN transition.
func (s *Service) runCheck(ctx context.Context, m Monitor) {
	timeout := time.Duration(m.TimeoutMs)*time.Millisecond + 2*time.Second
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := s.checker.Check(checkCtx, m)
	result.ID = uuid.NewString()

	dbResult := resultToDb(result)
	if err := s.store.InsertUptimeResult(ctx, dbResult); err != nil {
		s.logger.Warn("uptime: insert result", "monitor_id", m.ID, "error", err)
	}

	s.mu.Lock()
	prev, hasPrev := s.lastStatus[m.ID]
	s.lastStatus[m.ID] = result.Status
	s.mu.Unlock()

	if hasPrev && prev != StatusDown && result.Status == StatusDown {
		s.notify(ctx, TriggerUptimeDown, webhooks.Message{
			Title: "Monitor down: " + m.Name,
			Text:  "Target " + m.Target + " is unreachable. Error: " + result.Error,
		})
	}
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
