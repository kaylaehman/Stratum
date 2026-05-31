// Package uptime implements Uptime Kuma-style endpoint monitoring.
// It defines the domain models, the periodic checker, and uptime-% statistics.
// Persistence is delegated to the db.UptimeStore seam (SQLite-backed).
package uptime

import "time"

// CheckType is the protocol used for a monitor.
type CheckType string

const (
	CheckHTTP CheckType = "http"
	CheckTCP  CheckType = "tcp"
	CheckICMP CheckType = "icmp"
)

// CheckStatus is the outcome of one check execution.
type CheckStatus string

const (
	StatusUp       CheckStatus = "up"
	StatusDown     CheckStatus = "down"
	StatusDegraded CheckStatus = "degraded"
)

// Monitor is a configured endpoint to watch.
type Monitor struct {
	ID              string
	Name            string
	Type            CheckType
	Target          string
	IntervalSeconds int
	TimeoutMs       int
	Expected        string // HTTP: expected status code or body keyword; empty = any 2xx
	Enabled         bool
	NodeID          *string // nil = backend-originated check
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Result is one recorded check outcome.
type Result struct {
	ID             string
	MonitorID      string
	CheckedAt      time.Time
	Status         CheckStatus
	ResponseTimeMs int
	Error          string
}

// Stats contains precomputed uptime percentages for a monitor.
type Stats struct {
	MonitorID  string      `json:"monitor_id"`
	Current    CheckStatus `json:"current_status"`
	Uptime24h  float64     `json:"uptime_24h"`
	Uptime7d   float64     `json:"uptime_7d"`
	Uptime30d  float64     `json:"uptime_30d"`
	AvgRespMs  float64     `json:"avg_response_ms"`
}
