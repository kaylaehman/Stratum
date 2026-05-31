package db

import (
	"context"
	"time"
)

// AlertPolicy is a notification routing rule (Feature C6).
// Match and QuietHours are stored as nested JSON in the DB.
type AlertPolicy struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Enabled        bool               `json:"enabled"`
	MinSeverity    string             `json:"min_severity"`  // info|warning|critical
	Channels       []string           `json:"channels"`      // webhook IDs; empty = all channels (back-compat default)
	Match          AlertPolicyMatch   `json:"match"`
	QuietHours     *AlertQuietHours   `json:"quiet_hours,omitempty"`
	DedupWindowSec int                `json:"dedup_window_sec"`
	Escalate       *AlertEscalate     `json:"escalate,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// AlertPolicyMatch constrains which alerts a policy applies to.
type AlertPolicyMatch struct {
	Sources []string `json:"sources,omitempty"` // empty = all sources
	KeyGlob string   `json:"key_glob,omitempty"` // empty = all keys
}

// AlertQuietHours suppresses non-critical alerts within a time window.
type AlertQuietHours struct {
	StartMin     int    `json:"start_min"`      // minutes-of-day (0-1439)
	EndMin       int    `json:"end_min"`        // minutes-of-day (0-1439)
	Tz           string `json:"tz"`             // IANA timezone
	AllowCritical bool  `json:"allow_critical"` // bypass quiet hours for critical
}

// AlertEscalate re-routes an unacknowledged alert after a delay.
type AlertEscalate struct {
	AfterSec int      `json:"after_sec"`
	Channels []string `json:"channels"`
}

// AlertDelivery is one routing decision recorded in alert_deliveries.
type AlertDelivery struct {
	ID        string    `json:"id"`
	PolicyID  string    `json:"policy_id"`
	AlertKey  string    `json:"alert_key"`
	Severity  string    `json:"severity"`
	Channel   string    `json:"channel"`
	Status    string    `json:"status"` // delivered|suppressed_dedup|suppressed_quiet
	CreatedAt time.Time `json:"created_at"`
}

// AlertDeliveryStatus values.
const (
	AlertDeliveryStatusDelivered       = "delivered"
	AlertDeliveryStatusSuppressedDedup = "suppressed_dedup"
	AlertDeliveryStatusSuppressedQuiet = "suppressed_quiet"
)

// AlertPolicyStore is the narrow DB interface the alertpolicy package needs.
// Implemented by *sqlite.Store in db/sqlite/alertpolicy.go; NOT added to
// the central db.Store interface.
type AlertPolicyStore interface {
	ListAlertPolicies(ctx context.Context) ([]AlertPolicy, error)
	GetAlertPolicy(ctx context.Context, id string) (AlertPolicy, error)
	CreateAlertPolicy(ctx context.Context, p AlertPolicy) error
	UpdateAlertPolicy(ctx context.Context, p AlertPolicy) error
	DeleteAlertPolicy(ctx context.Context, id string) error

	InsertAlertDelivery(ctx context.Context, d AlertDelivery) error
	// ListAlertDeliveries returns at most limit rows newest-first (0 = default 100).
	ListAlertDeliveries(ctx context.Context, limit int) ([]AlertDelivery, error)
	// LatestDeliveryForKey returns the most recent delivered (non-suppressed) row
	// for the alert key across all policies, used for dedup window checks.
	LatestDeliveryForKey(ctx context.Context, alertKey string) (AlertDelivery, error)
}
