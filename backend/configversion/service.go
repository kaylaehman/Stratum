// Package configversion implements C3: config file versioning, drift detection,
// and revert. Each save of a tracked file (compose, key configs) produces a
// snapshot row keyed by (node_id, path). Drift compares the current on-disk
// content against the latest known-good snapshot. Revert writes a chosen
// snapshot back via the provided WriteFunc.
//
// Git-remote backend: the feature flag "feature.config_git" (default false)
// gates an optional git push after every snapshot. The interface GitBackend is
// stubbed here; a real implementation would use go-git or shell out to git.
package configversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/db"
)

// maxContentBytes is the hard cap on snapshot content size (1 MiB).
const maxContentBytes = 1 << 20

// ErrContentTooLarge is returned when file content exceeds maxContentBytes.
var ErrContentTooLarge = errors.New("configversion: content exceeds 1 MiB limit; snapshot skipped")

// ReadFunc fetches the current content of a file on a node over SSH/SFTP.
type ReadFunc func(ctx context.Context, nodeID, path string) ([]byte, error)

// WriteFunc writes content to a file on a node over SSH/SFTP.
type WriteFunc func(ctx context.Context, nodeID, path string, content []byte) error

// GitBackend is an optional interface for pushing snapshots to a remote git
// repository. The default no-op implementation satisfies the interface when
// feature.config_git is false (the default). A real implementation is
// left as a follow-on; the interface contract is stable.
type GitBackend interface {
	// Commit records the snapshot in the git backend.
	Commit(ctx context.Context, nodeID, path, hash, author string) error
}

// noopGit satisfies GitBackend with no side effects.
type noopGit struct{}

func (noopGit) Commit(_ context.Context, _, _, _, _ string) error { return nil }

// DriftResult holds a comparison between the current on-disk content and the
// latest known-good snapshot.
type DriftResult struct {
	NodeID          string `json:"node_id"`
	Path            string `json:"path"`
	HasSnapshot     bool   `json:"has_snapshot"`
	IsDrifted       bool   `json:"is_drifted"`
	SnapshotID      string `json:"snapshot_id,omitempty"`
	SnapshotHash    string `json:"snapshot_hash,omitempty"`
	CurrentHash     string `json:"current_hash,omitempty"`
	SnapshotContent string `json:"snapshot_content,omitempty"` // set when IsDrifted
	CurrentContent  string `json:"current_content,omitempty"`  // set when IsDrifted
}

// Service manages config-version snapshots and drift detection.
type Service struct {
	store  Store
	read   ReadFunc
	write  WriteFunc
	git    GitBackend
	gitOn  bool
}

// New wires the service. gitEnabled should mirror features.Service.Enabled for
// "feature.config_git"; pass false to use the no-op git stub.
func New(store Store, read ReadFunc, write WriteFunc, gitEnabled bool) *Service {
	var git GitBackend = noopGit{}
	return &Service{store: store, read: read, write: write, git: git, gitOn: gitEnabled}
}

// Snapshot takes a manual or automatic snapshot of the current file content.
// author is the user ID triggering the snapshot (empty string is accepted for
// automated triggers). Returns db.ErrNotFound if the file cannot be read, or
// ErrContentTooLarge if the file exceeds the 1 MiB cap.
func (s *Service) Snapshot(ctx context.Context, nodeID, path, author string) (db.ConfigVersion, error) {
	content, err := s.read(ctx, nodeID, path)
	if err != nil {
		return db.ConfigVersion{}, fmt.Errorf("configversion: read file: %w", err)
	}
	return s.snapshotContent(ctx, nodeID, path, content, author)
}

// SnapshotContent records a snapshot from already-read bytes (used by the
// FSWriteFile hook to avoid a redundant SSH round-trip).
func (s *Service) SnapshotContent(ctx context.Context, nodeID, path string, content []byte, author string) (db.ConfigVersion, error) {
	return s.snapshotContent(ctx, nodeID, path, content, author)
}

func (s *Service) snapshotContent(ctx context.Context, nodeID, path string, content []byte, author string) (db.ConfigVersion, error) {
	if int64(len(content)) > maxContentBytes {
		return db.ConfigVersion{}, ErrContentTooLarge
	}
	h := hashContent(content)
	v := db.ConfigVersion{
		ID:        uuid.NewString(),
		NodeID:    nodeID,
		Path:      path,
		Content:   string(content),
		Hash:      h,
		Author:    author,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.InsertConfigVersion(ctx, v); err != nil {
		return db.ConfigVersion{}, fmt.Errorf("configversion: insert: %w", err)
	}
	if s.gitOn {
		_ = s.git.Commit(ctx, nodeID, path, h, author) // best-effort; non-fatal
	}
	return v, nil
}

// History returns the version history for (nodeID, path), newest first.
// Content is included in each row.
func (s *Service) History(ctx context.Context, nodeID, path string) ([]db.ConfigVersion, error) {
	return s.store.ListConfigVersions(ctx, nodeID, path)
}

// Drift reads the current on-disk file and compares its hash to the latest
// snapshot. Returns DriftResult.IsDrifted=true and both contents when they
// differ. Returns HasSnapshot=false when no snapshot exists yet.
func (s *Service) Drift(ctx context.Context, nodeID, path string) (DriftResult, error) {
	res := DriftResult{NodeID: nodeID, Path: path}

	latest, err := s.store.LatestConfigVersion(ctx, nodeID, path)
	if errors.Is(err, db.ErrNotFound) {
		return res, nil // no baseline — not an error
	}
	if err != nil {
		return res, fmt.Errorf("configversion: fetch latest: %w", err)
	}
	res.HasSnapshot = true
	res.SnapshotID = latest.ID
	res.SnapshotHash = latest.Hash

	current, err := s.read(ctx, nodeID, path)
	if err != nil {
		return res, fmt.Errorf("configversion: read current: %w", err)
	}
	res.CurrentHash = hashContent(current)
	if res.CurrentHash == res.SnapshotHash {
		return res, nil
	}

	res.IsDrifted = true
	res.SnapshotContent = latest.Content
	res.CurrentContent = string(current)
	return res, nil
}

// Revert fetches a specific version by ID and writes its content back to the
// node via WriteFunc. The caller is responsible for auditing this action.
func (s *Service) Revert(ctx context.Context, nodeID, versionID string) (db.ConfigVersion, error) {
	v, err := s.store.GetConfigVersion(ctx, versionID)
	if err != nil {
		return db.ConfigVersion{}, err
	}
	if v.NodeID != nodeID {
		return db.ConfigVersion{}, db.ErrNotFound
	}
	if err := s.write(ctx, nodeID, v.Path, []byte(v.Content)); err != nil {
		return db.ConfigVersion{}, fmt.Errorf("configversion: write revert: %w", err)
	}
	return v, nil
}

// hashContent returns the hex-encoded SHA-256 of b.
func hashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
