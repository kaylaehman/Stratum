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
	"time"

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
}

// New wires the store + docker client provider.
func New(store db.Store, provider ClientProvider) *Service {
	return &Service{store: store, provider: provider}
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
	spec, err := client.CaptureSpec(ctx, ctr.DockerID)
	if err != nil {
		return "", fmt.Errorf("capture spec: %w", err)
	}
	if _, err := s.snapshotContainer(ctx, ctr, client, "pre-update"); err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	ref := spec.Config.Image
	if ref == "" {
		return "", fmt.Errorf("container has no image reference to pull")
	}
	if err := client.PullImage(ctx, ref); err != nil {
		return "", fmt.Errorf("pull %s: %w", ref, err)
	}
	newID, err := client.ApplySpec(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("recreate: %w", err)
	}
	return newID, nil
}

// Rollback restores a container from a snapshot: it pins the image to the
// snapshot's digest (when known), pulls it, and recreates. The container's
// CURRENT state is snapshotted first (best-effort) so a rollback is itself
// reversible. Returns the new docker container id.
func (s *Service) Rollback(ctx context.Context, snapshotID string) (string, error) {
	snap, err := s.store.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return "", err
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

	// Snapshot the current container (by name) before clobbering it, so the
	// rollback can itself be undone. Best-effort: the current container may be
	// broken/absent, which is exactly why we're rolling back.
	if cur, cerr := client.CaptureSpec(ctx, snap.ContainerName); cerr == nil {
		s.storeSpec(ctx, db.Container{NodeID: snap.NodeID, Name: snap.ContainerName, Image: spec.Config.Image}, cur, "", "pre-rollback")
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
