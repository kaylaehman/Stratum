package configversion_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/KAE-Labs/stratum/backend/configversion"
	"github.com/KAE-Labs/stratum/backend/db"
)

// --- in-memory Store stub ---

type memStore struct {
	rows []db.ConfigVersion
}

func (m *memStore) InsertConfigVersion(_ context.Context, v db.ConfigVersion) error {
	m.rows = append(m.rows, v)
	return nil
}

func (m *memStore) ListConfigVersions(_ context.Context, nodeID, path string) ([]db.ConfigVersion, error) {
	var out []db.ConfigVersion
	for _, v := range m.rows {
		if v.NodeID == nodeID && v.Path == path {
			out = append(out, v)
		}
	}
	// newest first
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (m *memStore) GetConfigVersion(_ context.Context, id string) (db.ConfigVersion, error) {
	for _, v := range m.rows {
		if v.ID == id {
			return v, nil
		}
	}
	return db.ConfigVersion{}, db.ErrNotFound
}

func (m *memStore) LatestConfigVersion(_ context.Context, nodeID, path string) (db.ConfigVersion, error) {
	var latest db.ConfigVersion
	found := false
	for _, v := range m.rows {
		if v.NodeID == nodeID && v.Path == path {
			if !found || v.CreatedAt.After(latest.CreatedAt) {
				latest = v
				found = true
			}
		}
	}
	if !found {
		return db.ConfigVersion{}, db.ErrNotFound
	}
	return latest, nil
}

func (m *memStore) DeleteConfigVersion(_ context.Context, id string) error {
	for i, v := range m.rows {
		if v.ID == id {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			return nil
		}
	}
	return db.ErrNotFound
}

// --- helpers ---

func makeService(store *memStore, disk map[string]string) *configversion.Service {
	read := func(_ context.Context, nodeID, path string) ([]byte, error) {
		if c, ok := disk[nodeID+":"+path]; ok {
			return []byte(c), nil
		}
		return nil, db.ErrNotFound
	}
	written := map[string]string{}
	write := func(_ context.Context, nodeID, path string, content []byte) error {
		written[nodeID+":"+path] = string(content)
		disk[nodeID+":"+path] = string(content)
		return nil
	}
	_ = written
	return configversion.New(store, read, write, false)
}

// --- tests ---

func TestHashDeterminism(t *testing.T) {
	// Two snapshots of the same content on the same node must produce the same hash.
	store := &memStore{}
	disk := map[string]string{"n1:/etc/foo": "hello"}
	svc := makeService(store, disk)
	ctx := context.Background()

	v1, err := svc.Snapshot(ctx, "n1", "/etc/foo", "user1")
	if err != nil {
		t.Fatalf("snapshot 1: %v", err)
	}
	v2, err := svc.Snapshot(ctx, "n1", "/etc/foo", "user1")
	if err != nil {
		t.Fatalf("snapshot 2: %v", err)
	}
	if v1.Hash != v2.Hash {
		t.Errorf("same content: hash mismatch %s vs %s", v1.Hash, v2.Hash)
	}
}

func TestHistoryOrdering(t *testing.T) {
	store := &memStore{}
	disk := map[string]string{"n1:/cfg": "v1"}
	svc := makeService(store, disk)
	ctx := context.Background()

	// Inject rows with known timestamps manually.
	store.rows = []db.ConfigVersion{
		{ID: uuid.NewString(), NodeID: "n1", Path: "/cfg", Content: "v1", Hash: "h1", Author: "u", CreatedAt: time.Now().Add(-2 * time.Minute)},
		{ID: uuid.NewString(), NodeID: "n1", Path: "/cfg", Content: "v2", Hash: "h2", Author: "u", CreatedAt: time.Now().Add(-1 * time.Minute)},
		{ID: uuid.NewString(), NodeID: "n1", Path: "/cfg", Content: "v3", Hash: "h3", Author: "u", CreatedAt: time.Now()},
	}

	history, err := svc.History(ctx, "n1", "/cfg")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("want 3 rows, got %d", len(history))
	}
	// Newest first.
	if history[0].Hash != "h3" {
		t.Errorf("want h3 first, got %s", history[0].Hash)
	}
	if history[2].Hash != "h1" {
		t.Errorf("want h1 last, got %s", history[2].Hash)
	}
}

func TestDriftNoDrift(t *testing.T) {
	store := &memStore{}
	disk := map[string]string{"n1:/etc/cfg": "same content"}
	svc := makeService(store, disk)
	ctx := context.Background()

	// Snapshot first.
	if _, err := svc.Snapshot(ctx, "n1", "/etc/cfg", "u"); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	// Disk unchanged — no drift expected.
	result, err := svc.Drift(ctx, "n1", "/etc/cfg")
	if err != nil {
		t.Fatalf("drift: %v", err)
	}
	if !result.HasSnapshot {
		t.Error("expected HasSnapshot=true")
	}
	if result.IsDrifted {
		t.Error("expected IsDrifted=false when content matches")
	}
}

func TestDriftDetected(t *testing.T) {
	store := &memStore{}
	disk := map[string]string{"n1:/etc/cfg": "original"}
	svc := makeService(store, disk)
	ctx := context.Background()

	if _, err := svc.Snapshot(ctx, "n1", "/etc/cfg", "u"); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	// Modify disk.
	disk["n1:/etc/cfg"] = "changed content"

	result, err := svc.Drift(ctx, "n1", "/etc/cfg")
	if err != nil {
		t.Fatalf("drift: %v", err)
	}
	if !result.IsDrifted {
		t.Error("expected IsDrifted=true after content change")
	}
	if result.SnapshotContent != "original" {
		t.Errorf("snapshot content: want 'original', got %q", result.SnapshotContent)
	}
	if result.CurrentContent != "changed content" {
		t.Errorf("current content: want 'changed content', got %q", result.CurrentContent)
	}
}

func TestDriftNoSnapshot(t *testing.T) {
	store := &memStore{}
	disk := map[string]string{"n1:/etc/cfg": "content"}
	svc := makeService(store, disk)
	ctx := context.Background()

	result, err := svc.Drift(ctx, "n1", "/etc/cfg")
	if err != nil {
		t.Fatalf("drift: %v", err)
	}
	if result.HasSnapshot {
		t.Error("expected HasSnapshot=false when no snapshot exists")
	}
	if result.IsDrifted {
		t.Error("expected IsDrifted=false when no snapshot exists")
	}
}

func TestRevertContent(t *testing.T) {
	store := &memStore{}
	disk := map[string]string{"n1:/etc/cfg": "original"}
	svc := makeService(store, disk)
	ctx := context.Background()

	v, err := svc.Snapshot(ctx, "n1", "/etc/cfg", "u")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	// Mutate disk.
	disk["n1:/etc/cfg"] = "modified"

	reverted, err := svc.Revert(ctx, "n1", v.ID)
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	if reverted.Content != "original" {
		t.Errorf("reverted content: want 'original', got %q", reverted.Content)
	}
	// Disk should be back to original.
	if disk["n1:/etc/cfg"] != "original" {
		t.Errorf("disk after revert: want 'original', got %q", disk["n1:/etc/cfg"])
	}
}

func TestRevertWrongNode(t *testing.T) {
	store := &memStore{}
	disk := map[string]string{"n1:/etc/cfg": "content"}
	svc := makeService(store, disk)
	ctx := context.Background()

	v, err := svc.Snapshot(ctx, "n1", "/etc/cfg", "u")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	// Revert on a different node should return not found.
	_, err = svc.Revert(ctx, "n2", v.ID)
	if err == nil {
		t.Error("expected error when reverting with wrong node_id")
	}
}

func TestSizeCapSkipsSnapshot(t *testing.T) {
	store := &memStore{}
	big := strings.Repeat("x", 1<<20+1)
	disk := map[string]string{"n1:/big": big}
	svc := makeService(store, disk)
	ctx := context.Background()

	_, err := svc.Snapshot(ctx, "n1", "/big", "u")
	if err == nil {
		t.Fatal("expected ErrContentTooLarge")
	}
	if len(store.rows) != 0 {
		t.Error("no row should be inserted when content exceeds size cap")
	}
}

func TestSnapshotContentDirectly(t *testing.T) {
	store := &memStore{}
	svc := makeService(store, nil)
	ctx := context.Background()

	content := []byte("some yaml content")
	v, err := svc.SnapshotContent(ctx, "n1", "/docker-compose.yml", content, "u")
	if err != nil {
		t.Fatalf("SnapshotContent: %v", err)
	}
	if v.Content != "some yaml content" {
		t.Errorf("content mismatch")
	}
	if len(v.Hash) != 64 {
		t.Errorf("expected 64-char SHA-256 hex, got %d chars", len(v.Hash))
	}
}
