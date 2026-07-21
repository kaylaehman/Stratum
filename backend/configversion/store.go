package configversion

import (
	"context"

	"github.com/KAE-Labs/stratum/backend/db"
)

// Store is the narrow persistence interface this package needs.
// It is implemented by *sqlite.Store without touching the central db.Store.
type Store interface {
	InsertConfigVersion(ctx context.Context, v db.ConfigVersion) error
	ListConfigVersions(ctx context.Context, nodeID, path string) ([]db.ConfigVersion, error)
	GetConfigVersion(ctx context.Context, id string) (db.ConfigVersion, error)
	LatestConfigVersion(ctx context.Context, nodeID, path string) (db.ConfigVersion, error)
	DeleteConfigVersion(ctx context.Context, id string) error
}
