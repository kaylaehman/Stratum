package uptime

import (
	"context"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
)

// Store is a subset of db.Store covering only uptime operations.
// It is a narrowing interface so the service can be tested without
// a full sqlite.Store mock.
type Store interface {
	CreateUptimeMonitor(ctx context.Context, m db.UptimeMonitor) error
	GetUptimeMonitor(ctx context.Context, id string) (db.UptimeMonitor, error)
	ListUptimeMonitors(ctx context.Context) ([]db.UptimeMonitor, error)
	UpdateUptimeMonitor(ctx context.Context, m db.UptimeMonitor) error
	DeleteUptimeMonitor(ctx context.Context, id string) error
	InsertUptimeResult(ctx context.Context, r db.UptimeResult) error
	ListUptimeResults(ctx context.Context, monitorID string, from, to time.Time) ([]db.UptimeResult, error)
	LatestUptimeResult(ctx context.Context, monitorID string) (db.UptimeResult, error)
	PruneUptimeResultsBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
