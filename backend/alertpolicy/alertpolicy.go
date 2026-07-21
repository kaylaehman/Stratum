// Package alertpolicy implements C6: alert policy / notification routing.
// It sits between alert emission (incident/webhooks) and channel dispatch,
// deciding per-policy whether to deliver, suppress (dedup / quiet-hours),
// or skip based on severity / match filters.
//
// Call Route(ctx, Alert) to get a []Decision; the caller dispatches to the
// webhook IDs listed in each decision.
package alertpolicy

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

// Alert is an emitted notification event passed to Route.
type Alert struct {
	// Key is the dedup key — typically "<trigger>/<target-id>".
	Key      string
	Severity string // info | warning | critical
	Title    string
	Text     string
	Source   string
}

// Decision is one routing outcome for one (policy, channel) pair.
type Decision struct {
	PolicyID  string
	Channel   string // webhook id; empty string = "all channels" (back-compat default)
	Status    string // delivered | suppressed_dedup | suppressed_quiet
}

// Store is the narrow DB interface the package needs.
// It is a superset of appdb.AlertPolicyStore, also including the delivery read
// methods required by routing. Satisfied by *sqlite.Store.
type Store interface {
	ListAlertPolicies(ctx context.Context) ([]appdb.AlertPolicy, error)
	GetAlertPolicy(ctx context.Context, id string) (appdb.AlertPolicy, error)
	CreateAlertPolicy(ctx context.Context, p appdb.AlertPolicy) error
	UpdateAlertPolicy(ctx context.Context, p appdb.AlertPolicy) error
	DeleteAlertPolicy(ctx context.Context, id string) error
	InsertAlertDelivery(ctx context.Context, d appdb.AlertDelivery) error
	ListAlertDeliveries(ctx context.Context, limit int) ([]appdb.AlertDelivery, error)
	LatestDeliveryForKey(ctx context.Context, alertKey string) (appdb.AlertDelivery, error)
}

// Service routes alerts through stored policies and records deliveries.
type Service struct {
	store Store
	now   func() time.Time // injectable for tests
}

// New creates a Service backed by the given store.
func New(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// Route evaluates all enabled policies against a and records delivery rows.
// Returns one Decision per (policy × channel) outcome.
// Errors from the DB are returned but a partial result may still be populated
// from policies evaluated before the failure.
func (s *Service) Route(ctx context.Context, a Alert) ([]Decision, error) {
	policies, err := s.store.ListAlertPolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("alertpolicy: load policies: %w", err)
	}

	now := s.now()

	// Compute latest delivery for dedup once (shared across policies).
	latest, latestErr := s.store.LatestDeliveryForKey(ctx, a.Key)
	if latestErr != nil && !errors.Is(latestErr, appdb.ErrNotFound) {
		return nil, fmt.Errorf("alertpolicy: latest delivery: %w", latestErr)
	}

	var decisions []Decision

	for _, p := range policies {
		if !p.Enabled {
			continue
		}
		if !matchesSeverity(p.MinSeverity, a.Severity) {
			continue
		}
		if !matchesFilter(p.Match, a) {
			continue
		}

		// Determine channels: empty slice on the default policy means "all channels"
		// (back-compat fire-hose — the caller resolves this to the full webhook list).
		channels := p.Channels
		if len(channels) == 0 {
			channels = []string{""} // sentinel: "all channels"
		}

		for _, ch := range channels {
			status, reason := s.classify(p, a, now, latest, latestErr)
			d := appdb.AlertDelivery{
				ID:        uuid.NewString(),
				PolicyID:  p.ID,
				AlertKey:  a.Key,
				Severity:  a.Severity,
				Channel:   ch,
				Status:    status,
				CreatedAt: now,
			}
			_ = reason // reason is for logging if desired; status is the canonical field
			if err := s.store.InsertAlertDelivery(ctx, d); err != nil {
				return decisions, fmt.Errorf("alertpolicy: record delivery: %w", err)
			}
			decisions = append(decisions, Decision{
				PolicyID: p.ID,
				Channel:  ch,
				Status:   status,
			})
		}
	}

	return decisions, nil
}

// ListPolicies returns all stored policies (for the CRUD API).
func (s *Service) ListPolicies(ctx context.Context) ([]appdb.AlertPolicy, error) {
	return s.store.ListAlertPolicies(ctx)
}

// ListDeliveries returns at most limit delivery rows newest-first.
func (s *Service) ListDeliveries(ctx context.Context, limit int) ([]appdb.AlertDelivery, error) {
	return s.store.ListAlertDeliveries(ctx, limit)
}

// StoreRef returns the underlying Store so the API handler can call CRUD
// methods (CreateAlertPolicy, UpdateAlertPolicy, DeleteAlertPolicy) without
// going through the routing path.
func (s *Service) StoreRef() Store { return s.store }

// classify returns the routing status for one (policy, channel) evaluation.
func (s *Service) classify(
	p appdb.AlertPolicy,
	a Alert,
	now time.Time,
	latest appdb.AlertDelivery,
	latestErr error,
) (status string, reason string) {
	// Dedup check: suppress if a delivered row exists within the dedup window.
	if p.DedupWindowSec > 0 && !errors.Is(latestErr, appdb.ErrNotFound) {
		age := now.Sub(latest.CreatedAt)
		if age < time.Duration(p.DedupWindowSec)*time.Second {
			return appdb.AlertDeliveryStatusSuppressedDedup, "dedup_window"
		}
	}

	// Quiet-hours check: suppress non-critical (or critical when AllowCritical=false).
	if p.QuietHours != nil {
		if inQuietHours(*p.QuietHours, now) {
			if !p.QuietHours.AllowCritical || a.Severity != "critical" {
				return appdb.AlertDeliveryStatusSuppressedQuiet, "quiet_hours"
			}
		}
	}

	return appdb.AlertDeliveryStatusDelivered, ""
}

// inQuietHours reports whether now falls within the quiet-hours window.
// StartMin and EndMin are minutes-of-day (0=00:00, 1439=23:59).
// The window may wrap midnight (StartMin > EndMin).
func inQuietHours(qh appdb.AlertQuietHours, now time.Time) bool {
	loc, err := time.LoadLocation(qh.Tz)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	minuteOfDay := local.Hour()*60 + local.Minute()

	start, end := qh.StartMin, qh.EndMin
	if start <= end {
		return minuteOfDay >= start && minuteOfDay < end
	}
	// wraps midnight
	return minuteOfDay >= start || minuteOfDay < end
}

// matchesSeverity reports whether alertSev meets the policy's minimum.
// Order: info < warning < critical. An unknown severity admits the alert.
func matchesSeverity(minSev, alertSev string) bool {
	order := map[string]int{"info": 0, "warning": 1, "critical": 2}
	minRank, ok := order[minSev]
	if !ok {
		return true
	}
	alertRank, ok := order[alertSev]
	if !ok {
		return true
	}
	return alertRank >= minRank
}

// matchesFilter reports whether the alert matches the policy's optional filters.
// An empty filter matches everything.
func matchesFilter(m appdb.AlertPolicyMatch, a Alert) bool {
	if len(m.Sources) > 0 {
		found := false
		for _, s := range m.Sources {
			if strings.EqualFold(s, a.Source) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if m.KeyGlob != "" {
		matched, err := path.Match(m.KeyGlob, a.Key)
		if err != nil || !matched {
			return false
		}
	}
	return true
}
