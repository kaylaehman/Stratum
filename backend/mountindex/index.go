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
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
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
}

// New builds an Index. ttl bounds how stale a node's mount rows may be before a
// query re-inspects (e.g. 30s).
func New(store db.Store, provider ClientProvider, ttl time.Duration) *Index {
	return &Index{store: store, provider: provider, ttl: ttl, seeded: map[string]time.Time{}}
}

// ensureFresh re-inspects a node's containers and refreshes their mount rows if
// the node hasn't been seeded within the TTL.
func (ix *Index) ensureFresh(ctx context.Context, nodeID string) error {
	ix.mu.Lock()
	if t, ok := ix.seeded[nodeID]; ok && time.Since(t) < ix.ttl {
		ix.mu.Unlock()
		return nil
	}
	ix.mu.Unlock()

	containers, err := ix.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return err
	}
	client, err := ix.provider(ctx, nodeID)
	if err != nil {
		return err
	}
	for _, c := range containers {
		info, err := client.Inspect(ctx, c.DockerID)
		if err != nil {
			continue // skip a container that vanished mid-seed; its rows cascade-clean on delete
		}
		_ = ix.store.ReplaceContainerMounts(ctx, c.ID, rowsFor(c, info.Mounts))
	}

	ix.mu.Lock()
	ix.seeded[nodeID] = time.Now()
	ix.mu.Unlock()
	return nil
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
