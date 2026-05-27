// Package updates implements the detection half of the Update Assistant
// (Feature 15): for each running container it compares the locally-present image
// repo digest against the registry's current manifest digest and classifies the
// container as up-to-date, update-available, or unknown. Registry lookups are
// slow/rate-limited, so results are cached per container with a TTL and seeded
// on query. The one-click update action (pull + recreate) is intentionally a
// separate, later concern — this package is read-only.
package updates

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// Status values.
const (
	StatusUpToDate        = "up_to_date"
	StatusUpdateAvailable = "update_available"
	StatusUnknown         = "unknown"
)

// perImageTimeout bounds a single registry lookup so one slow/unreachable
// registry can't stall a whole node's check.
const perImageTimeout = 8 * time.Second

// Service checks and caches image-update status.
type Service struct {
	store    db.Store
	provider ClientProvider
	ttl      time.Duration

	mu     sync.Mutex
	seeded map[string]time.Time
	sf     singleflight.Group
}

// New builds the service. ttl bounds how stale a node's cached results may be
// before a query re-checks (registry calls are slow — use hours).
func New(store db.Store, provider ClientProvider, ttl time.Duration) *Service {
	return &Service{store: store, provider: provider, ttl: ttl, seeded: map[string]time.Time{}}
}

func (s *Service) fresh(nodeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.seeded[nodeID]
	return ok && time.Since(t) < s.ttl
}

// Invalidate forces the next EnsureFresh for a node to re-check.
func (s *Service) Invalidate(nodeID string) {
	s.mu.Lock()
	delete(s.seeded, nodeID)
	s.mu.Unlock()
}

// EnsureFresh re-checks a node's running containers if the cache is stale
// (singleflight-deduped). Best-effort per container; a failing registry lookup
// yields status "unknown" rather than aborting.
func (s *Service) EnsureFresh(ctx context.Context, nodeID string) error {
	if s.fresh(nodeID) {
		return nil
	}
	_, err, _ := s.sf.Do(nodeID, func() (any, error) {
		if s.fresh(nodeID) {
			return nil, nil
		}
		containers, err := s.store.ListContainersByNode(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		client, err := s.provider(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		for _, c := range containers {
			if c.Status != "running" {
				continue
			}
			s.checkOne(ctx, client, c)
		}
		s.mu.Lock()
		s.seeded[nodeID] = time.Now()
		s.mu.Unlock()
		return nil, nil
	})
	return err
}

func (s *Service) checkOne(ctx context.Context, client *docker.Client, c db.Container) {
	cctx, cancel := context.WithTimeout(ctx, perImageTimeout)
	defer cancel()
	local, _ := client.LocalRepoDigest(cctx, c.ImageID)
	remote, rerr := client.RemoteDigest(cctx, c.Image)
	status := Classify(local, remote, rerr != nil)
	_ = s.store.UpsertImageUpdate(ctx, db.ImageUpdateRow{
		ContainerID: c.ID, NodeID: c.NodeID, Image: c.Image,
		Status: status, CurrentDigest: local, LatestDigest: remote, CheckedAt: time.Now(),
	})
}

// ListAll returns the cached update rows across all nodes.
func (s *Service) ListAll(ctx context.Context) ([]db.ImageUpdateRow, error) {
	return s.store.ListImageUpdates(ctx)
}

// EnsureAll re-checks every docker-capable node (best-effort).
func (s *Service) EnsureAll(ctx context.Context) {
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return
	}
	for _, n := range nodes {
		caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
		if caps.Docker {
			_ = s.EnsureFresh(ctx, n.ID)
		}
	}
}

// Classify decides update status from the local repo digest, the remote
// manifest digest, and whether the remote lookup failed. Unknown when either
// digest is unavailable (locally-built image, private/rate-limited registry).
func Classify(localDigest, remoteDigest string, remoteFailed bool) string {
	if localDigest == "" || remoteFailed || remoteDigest == "" {
		return StatusUnknown
	}
	if localDigest == remoteDigest {
		return StatusUpToDate
	}
	return StatusUpdateAvailable
}
