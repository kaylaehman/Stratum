// Package recreate implements the destructive half of the Update Assistant
// (Feature 15) and Rollback (Feature 17): it captures a container's full
// create-spec into a rollback snapshot, then recreates the container — either
// pulling the latest image (update) or restoring a snapshotted image+config
// (rollback). The actual daemon choreography (crash-safe rename→create→start,
// restore-on-failure) lives in docker.Client.ApplySpec; this layer owns
// snapshotting, retention, and image-reference resolution.
package recreate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
)

// keepSnapshots is the per-container rollback retention (CLAUDE: "last 10").
const keepSnapshots = 10

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// Service owns snapshot + recreate operations.
type Service struct {
	store    db.Store
	provider ClientProvider

	// locks serialise all recreate/snapshot work per container (keyed by
	// node/name) so two concurrent updates can't race the rename→recreate
	// choreography (e.g. clobber each other's backup). Single-process backend.
	locks sync.Map // "nodeID/name" -> *sync.Mutex
}

// New wires the store + docker client provider.
func New(store db.Store, provider ClientProvider) *Service {
	return &Service{store: store, provider: provider}
}

// lock serialises operations for one container and returns the unlock func.
func (s *Service) lock(nodeID, name string) func() {
	mui, _ := s.locks.LoadOrStore(nodeID+"/"+name, &sync.Mutex{})
	mu := mui.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// List returns a container's rollback snapshots (newest first). The container is
// identified by our app id; snapshots are keyed by (node, name) so they survive
// the docker-id churn of a recreate.
func (s *Service) List(ctx context.Context, containerID string) ([]db.Snapshot, error) {
	ctr, err := s.store.GetContainer(ctx, containerID)
	if err != nil {
		return nil, err
	}
	return s.store.ListSnapshotsByContainer(ctx, ctr.NodeID, ctr.Name)
}

// Snapshot captures the current create-spec of a container into a new rollback
// snapshot (manual "Save checkpoint" or pre-change). Returns the stored row.
func (s *Service) Snapshot(ctx context.Context, containerID, reason string) (db.Snapshot, error) {
	ctr, client, err := s.resolve(ctx, containerID)
	if err != nil {
		return db.Snapshot{}, err
	}
	unlock := s.lock(ctr.NodeID, ctr.Name)
	defer unlock()
	return s.snapshotContainer(ctx, ctr, client, reason)
}

// Update pulls the latest image for the container's configured reference, then
// recreates it with the same config. A pre-update snapshot is taken first so the
// change is reversible. Returns the new docker container id.
func (s *Service) Update(ctx context.Context, containerID string) (string, error) {
	ctr, client, err := s.resolve(ctx, containerID)
	if err != nil {
		return "", err
	}
	unlock := s.lock(ctr.NodeID, ctr.Name)
	defer unlock()

	spec, err := client.CaptureSpec(ctx, ctr.DockerID)
	if err != nil {
		return "", fmt.Errorf("capture spec: %w", err)
	}
	ref := spec.Config.Image
	if ref == "" {
		return "", fmt.Errorf("container has no image reference to pull")
	}
	snap, err := s.snapshotContainer(ctx, ctr, client, "pre-update")
	if err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	// If the pull fails, nothing destructive happened yet — drop the pre-update
	// snapshot so repeated failed updates don't evict real checkpoints. (If the
	// recreate itself fails below, we KEEP the snapshot: it's the recovery path.)
	if err := client.PullImage(ctx, ref); err != nil {
		_ = s.store.DeleteSnapshot(ctx, snap.ID)
		return "", fmt.Errorf("pull %s: %w", ref, err)
	}
	newID, err := client.ApplySpec(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("recreate: %w", err)
	}
	return newID, nil
}

// ErrSnapshotMismatch is returned when a snapshot does not belong to the
// container the rollback was requested against.
var ErrSnapshotMismatch = fmt.Errorf("recreate: snapshot does not belong to this container")

// HealthcheckEdit is a requested healthcheck change (Feature 5). Disable wins:
// when set, the container's healthcheck is turned off ({"NONE"}). Otherwise Test
// (e.g. ["CMD-SHELL","curl -f localhost:8096 || exit 1"]) and the timing fields
// define the new check. Zero timing fields fall back to Docker's defaults.
type HealthcheckEdit struct {
	Disable        bool
	Test           []string
	IntervalSec    int
	TimeoutSec     int
	StartPeriodSec int
	Retries        int
}

// SetHealthcheck edits a container's healthcheck and recreates it (Docker can't
// change a healthcheck on a live container). A pre-edit snapshot is taken first.
// Returns the new docker container id.
func (s *Service) SetHealthcheck(ctx context.Context, containerID string, e HealthcheckEdit) (string, error) {
	ctr, client, err := s.resolve(ctx, containerID)
	if err != nil {
		return "", err
	}
	unlock := s.lock(ctr.NodeID, ctr.Name)
	defer unlock()

	spec, err := client.CaptureSpec(ctx, ctr.DockerID)
	if err != nil {
		return "", fmt.Errorf("capture spec: %w", err)
	}
	if spec.Config == nil {
		return "", fmt.Errorf("container has no config to edit")
	}
	digest, _ := client.CurrentImageDigest(ctx, ctr.DockerID)
	// storeSpec marshals the spec NOW, so the snapshot captures the pre-edit
	// healthcheck even though we mutate spec.Config below.
	if _, err := s.storeSpec(ctx, ctr, spec, digest, "pre-healthcheck"); err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}

	if e.Disable {
		spec.Config.Healthcheck = &container.HealthConfig{Test: []string{"NONE"}}
	} else {
		spec.Config.Healthcheck = &container.HealthConfig{
			Test:        e.Test,
			Interval:    time.Duration(e.IntervalSec) * time.Second,
			Timeout:     time.Duration(e.TimeoutSec) * time.Second,
			StartPeriod: time.Duration(e.StartPeriodSec) * time.Second,
			Retries:     e.Retries,
		}
	}

	newID, err := client.ApplySpec(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("recreate: %w", err)
	}
	return newID, nil
}

// Rollback restores a container from a snapshot: it pins the image to the
// snapshot's digest (when known), pulls it, and recreates. wantNodeID/wantName
// identify the container the caller is rolling back; the snapshot MUST belong to
// it (guards against replaying one container's spec onto another). The
// container's CURRENT state is snapshotted first (best-effort) so a rollback is
// itself reversible. Returns the new docker container id.
func (s *Service) Rollback(ctx context.Context, snapshotID, wantNodeID, wantName string) (string, error) {
	snap, err := s.store.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return "", err
	}
	if snap.NodeID != wantNodeID || snap.ContainerName != wantName {
		return "", ErrSnapshotMismatch
	}
	client, err := s.provider(ctx, snap.NodeID)
	if err != nil {
		return "", err
	}
	var spec docker.RecreateSpec
	if err := json.Unmarshal([]byte(snap.SpecJSON), &spec); err != nil {
		return "", fmt.Errorf("decode snapshot spec: %w", err)
	}
	if spec.Config == nil || spec.Name == "" {
		return "", fmt.Errorf("snapshot spec is incomplete")
	}

	unlock := s.lock(snap.NodeID, snap.ContainerName)
	defer unlock()

	// Snapshot the current container (by name) before clobbering it, so the
	// rollback can itself be undone. Best-effort: the current container may be
	// broken/absent, which is exactly why we're rolling back. Use the CURRENT
	// container's own image ref (cur.Config.Image), not the rollback target's.
	if cur, cerr := client.CaptureSpec(ctx, snap.ContainerName); cerr == nil {
		curImage := snap.ContainerName
		if cur.Config != nil {
			curImage = cur.Config.Image
		}
		s.storeSpec(ctx, db.Container{NodeID: snap.NodeID, Name: snap.ContainerName, Image: curImage}, cur, "", "pre-rollback")
	}

	// Pin the image to the snapshotted digest when we have one, so we restore the
	// exact image and not whatever the tag now points to.
	if pinned := pinnedRef(snap.ImageRef, snap.ImageDigest); pinned != "" {
		if err := client.PullImage(ctx, pinned); err != nil {
			return "", fmt.Errorf("pull %s: %w", pinned, err)
		}
		spec.Config.Image = pinned
	}

	newID, err := client.ApplySpec(ctx, &spec)
	if err != nil {
		return "", fmt.Errorf("recreate: %w", err)
	}
	return newID, nil
}

// resolve loads the container row and a docker client for its node.
func (s *Service) resolve(ctx context.Context, containerID string) (db.Container, *docker.Client, error) {
	ctr, err := s.store.GetContainer(ctx, containerID)
	if err != nil {
		return db.Container{}, nil, err
	}
	client, err := s.provider(ctx, ctr.NodeID)
	if err != nil {
		return db.Container{}, nil, err
	}
	return ctr, client, nil
}

// snapshotContainer captures the live spec for a container row and stores it.
func (s *Service) snapshotContainer(ctx context.Context, ctr db.Container, client *docker.Client, reason string) (db.Snapshot, error) {
	spec, err := client.CaptureSpec(ctx, ctr.DockerID)
	if err != nil {
		return db.Snapshot{}, fmt.Errorf("capture spec: %w", err)
	}
	digest, _ := client.CurrentImageDigest(ctx, ctr.DockerID) // best-effort; empty for locally-built images
	return s.storeSpec(ctx, ctr, spec, digest, reason)
}

// storeSpec marshals a spec and writes a snapshot row, pruning to the retention
// limit. It is the single write path for snapshots.
func (s *Service) storeSpec(ctx context.Context, ctr db.Container, spec *docker.RecreateSpec, digest, reason string) (db.Snapshot, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return db.Snapshot{}, fmt.Errorf("encode spec: %w", err)
	}
	snap := db.Snapshot{
		ID:            uuid.NewString(),
		ContainerID:   ctr.ID,
		NodeID:        ctr.NodeID,
		ContainerName: ctr.Name,
		Reason:        reason,
		ImageRef:      ctr.Image,
		ImageDigest:   digest,
		SpecJSON:      string(raw),
		CreatedAt:     time.Now(),
	}
	if err := s.store.CreateSnapshot(ctx, snap); err != nil {
		return db.Snapshot{}, err
	}
	_ = s.store.PruneSnapshots(ctx, ctr.NodeID, ctr.Name, keepSnapshots)
	return snap, nil
}

// pinnedRef builds a digest-pinned reference ("repo@sha256:...") from a tagged
// reference + digest. Returns "" when either input is missing (caller then
// recreates with the spec's existing image reference unchanged).
func pinnedRef(ref, digest string) string {
	if ref == "" || digest == "" {
		return ""
	}
	if strings.Contains(ref, "@") {
		return ref // already digest-pinned
	}
	return repoOnly(ref) + "@" + digest
}

// repoOnly strips a tag from an image reference, keeping any registry host:port.
// "ghcr.io:443/org/app:v1" -> "ghcr.io:443/org/app"; "nginx:latest" -> "nginx".
func repoOnly(ref string) string {
	slash := strings.LastIndex(ref, "/")
	colon := strings.LastIndex(ref, ":")
	if colon > slash { // the last colon is a tag separator, not a registry port
		return ref[:colon]
	}
	return ref
}
