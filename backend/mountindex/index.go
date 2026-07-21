// Package mountindex maintains a per-node index of container mounts (bind +
// volume) so the bind-mount tracer can answer forward (by container), reverse
// (by host path), and shared (multi-container) queries without inspecting every
// container per request. The index is seeded on query with a TTL (re-inspecting
// a node's containers when stale); rows cascade-delete with their container/node.
package mountindex

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/docker"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// Index seeds and queries the mounts table.
type Index struct {
	store    db.Store
	provider ClientProvider
	ttl      time.Duration

	mu     sync.Mutex
	seeded map[string]time.Time
	sf     singleflight.Group // dedupe concurrent seeds for the same node
}

// New builds an Index. ttl bounds how stale a node's mount rows may be before a
// query re-inspects (e.g. 30s).
func New(store db.Store, provider ClientProvider, ttl time.Duration) *Index {
	return &Index{store: store, provider: provider, ttl: ttl, seeded: map[string]time.Time{}}
}

// ensureFresh re-inspects a node's containers and refreshes their mount rows if
// the node hasn't been seeded within the TTL.
func (ix *Index) fresh(nodeID string) bool {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	t, ok := ix.seeded[nodeID]
	return ok && time.Since(t) < ix.ttl
}

func (ix *Index) ensureFresh(ctx context.Context, nodeID string) error {
	if ix.fresh(nodeID) {
		return nil
	}
	// singleflight dedupes concurrent seeds for the same node (avoids racing
	// delete+insert transactions and double inspects).
	_, err, _ := ix.sf.Do(nodeID, func() (any, error) {
		if ix.fresh(nodeID) { // another goroutine seeded while we waited
			return nil, nil
		}
		containers, err := ix.store.ListContainersByNode(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		client, err := ix.provider(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		var firstErr error
		for _, c := range containers {
			info, err := client.Inspect(ctx, c.DockerID)
			if err != nil {
				continue // skip a container that vanished mid-seed; rows cascade-clean on delete
			}
			if rerr := ix.store.ReplaceContainerMounts(ctx, c.ID, rowsFor(c, info.Mounts)); rerr != nil && firstErr == nil {
				firstErr = rerr
			}
		}
		if firstErr != nil {
			return nil, firstErr // do NOT mark seeded on a persist error — retry next query
		}
		ix.mu.Lock()
		ix.seeded[nodeID] = time.Now()
		ix.mu.Unlock()
		return nil, nil
	})
	return err
}

// EnsureFresh re-seeds a node's mount rows if stale (exported so other features
// — e.g. volume health — can rely on the persisted mount index being current
// before reading it directly via the store).
func (ix *Index) EnsureFresh(ctx context.Context, nodeID string) error {
	return ix.ensureFresh(ctx, nodeID)
}

// Invalidate forces the next query for a node to re-seed.
func (ix *Index) Invalidate(nodeID string) {
	ix.mu.Lock()
	delete(ix.seeded, nodeID)
	ix.mu.Unlock()
}

// rowsFor converts a container's inspect mounts into index rows.
func rowsFor(c db.Container, mounts []docker.Mount) []db.MountRow {
	rows := make([]db.MountRow, 0, len(mounts))
	for _, m := range mounts {
		rows = append(rows, db.MountRow{
			ID:               uuid.NewString(),
			NodeID:           c.NodeID,
			ContainerID:      c.ID,
			Type:             m.Type,
			Source:           m.Source,
			NormalizedSource: path.Clean(m.Source),
			VolumeName:       m.Name,
			Destination:      m.Destination,
			RW:               m.RW,
		})
	}
	return rows
}
