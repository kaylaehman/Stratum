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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/KAE-Labs/stratum/backend/capabilities"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/docker"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// NotifyFunc is called when a running container's image has an update available.
// Signature matches the existing SetNotify pattern used across services.
type NotifyFunc func(ctx context.Context, trigger, title, text string)

// triggerImageUpdateAvail is the webhook trigger key for image-update alerts.
const triggerImageUpdateAvail = "image.update_available"

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
	notify   NotifyFunc // may be nil; update notifications are best-effort

	mu     sync.Mutex
	seeded map[string]time.Time
	sf     singleflight.Group
}

// New builds the service. ttl bounds how stale a node's cached results may be
// before a query re-checks (registry calls are slow — use hours).
func New(store db.Store, provider ClientProvider, ttl time.Duration) *Service {
	return &Service{store: store, provider: provider, ttl: ttl, seeded: map[string]time.Time{}}
}

// SetNotify wires the notification callback. Called during server startup.
// If nil, image-update notifications are silently skipped.
func (s *Service) SetNotify(fn NotifyFunc) { s.notify = fn }

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

// checkOne fetches the local repo digest and the remote manifest digest for one
// container and persists the result. Errors from either digest fetch are logged
// and surfaced as UnknownReason so operators can see why a check failed.
func (s *Service) checkOne(ctx context.Context, client *docker.Client, c db.Container) {
	cctx, cancel := context.WithTimeout(ctx, perImageTimeout)
	defer cancel()

	// ImageID may be empty for containers inventoried before the field existed;
	// fall back to the image name/ref in that case.
	lookupID := c.ImageID
	if lookupID == "" {
		lookupID = c.Image
	}

	local, localErr := client.LocalRepoDigest(cctx, lookupID)
	if localErr != nil {
		slog.Warn("updates: local digest lookup failed",
			"container", c.Name, "image_id", lookupID, "err", localErr)
	}

	remote, remoteErr := client.RemoteDigest(cctx, c.Image)
	if remoteErr != nil {
		slog.Warn("updates: remote digest lookup failed",
			"container", c.Name, "image", c.Image, "err", remoteErr)
	}

	newStatus, unknownReason := Classify(local, remote, localErr, remoteErr)

	if err := s.store.UpsertImageUpdate(ctx, db.ImageUpdateRow{
		ContainerID:   c.ID,
		NodeID:        c.NodeID,
		Image:         c.Image,
		Status:        newStatus,
		CurrentDigest: local,
		LatestDigest:  remote,
		UnknownReason: unknownReason,
		CheckedAt:     time.Now(),
	}); err != nil {
		slog.Error("updates: failed to persist image update row",
			"container", c.Name, "err", err)
	}

	// Fire a notification when an update is detected. The webhook dispatcher's
	// 5-minute rate window prevents alert floods when EnsureFresh runs frequently.
	if newStatus == StatusUpdateAvailable && s.notify != nil {
		s.notify(ctx, triggerImageUpdateAvail,
			"Image update available",
			fmt.Sprintf("Container %q (%s) has a newer image available", c.Name, c.Image),
		)
	}
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
// manifest digest, and any errors from fetching them. It returns the status
// string and, when the status is "unknown", a human-readable explanation of why
// the comparison could not be made.
//
// Rules (evaluated in order):
//   - localErr != nil  → unknown: daemon / image-inspect failure
//   - localDigest == "" → unknown: locally-built image or no repo digest
//   - remoteErr != nil → unknown: registry unreachable / auth / rate-limited
//   - remoteDigest == "" → unknown: registry returned an empty digest
//   - local == remote  → up_to_date
//   - otherwise        → update_available
func Classify(localDigest, remoteDigest string, localErr, remoteErr error) (status, unknownReason string) {
	switch {
	case localErr != nil:
		return StatusUnknown, fmt.Sprintf("local digest unavailable: %v", localErr)
	case localDigest == "":
		return StatusUnknown, "no repo digest (locally-built or never pushed)"
	case remoteErr != nil:
		return StatusUnknown, fmt.Sprintf("registry lookup failed: %v", remoteErr)
	case remoteDigest == "":
		return StatusUnknown, "registry returned empty digest"
	case localDigest == remoteDigest:
		return StatusUpToDate, ""
	default:
		return StatusUpdateAvailable, ""
	}
}
