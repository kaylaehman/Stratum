// Package uptime implements Uptime Kuma-style endpoint monitoring.
// It defines the domain models, the periodic checker, and uptime-% statistics.
// Persistence is delegated to the db.UptimeStore seam (SQLite-backed).
//
// # Check types
//
//   Type   Mechanism                   Notes
//   ──────────────────────────────────────────────────────────────────────
//   http   stdlib http.Client GET      status 2xx/3xx = up; >80% of timeout = degraded
//   tcp    net.Dialer                  connect success = up; error = down
//   icmp   TCP port-7 + port-80 + udp  approximation; raw ICMP needs elevated privs
//
// # HTTP classification
//
//   HTTP 2xx/3xx + elapsed ≤ 80% of timeoutMs  → up
//   HTTP 2xx/3xx + elapsed  > 80% of timeoutMs  → degraded
//   HTTP 4xx/5xx                                 → down
//   Network error / timeout                      → down
//
// Optional keyword check: when Monitor.Expected is a non-numeric string, the
// response body (up to 64 KiB) is scanned for that substring. Mismatch → down.
// When Expected is a 3-digit numeric string (e.g. "200") the keyword check is
// skipped entirely to avoid false negatives on numeric body content.
//
// # Alert state machine
//
// DOWN is declared only after DefaultConsecutiveFailuresRequired (2) consecutive
// failed probes. A single transient failure never fires a webhook. Recovery
// (any successful probe) resets the counter so future failures can re-alert.
// Degraded counts as a failure for the purpose of the consecutive-failure gate.
//
// # Uptime percentage
//
// UptimePct counts checks with status "up" OR "degraded" as "up" for % purposes.
// Windows that have received no results return 100% (no-data = assume up).
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
