package recreate

import (
	"context"
	"testing"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/docker"
)

// mismatchStore returns a snapshot belonging to a fixed (node, name).
type mismatchStore struct {
	db.Store
	snap db.Snapshot
}

func (m *mismatchStore) GetSnapshot(context.Context, string) (db.Snapshot, error) {
	return m.snap, nil
}

// TestRollbackRejectsForeignSnapshot guards C1: a snapshot from one container
// must not be replayable against another. The ownership check runs before the
// docker client is ever requested, so a nil provider proves it never gets there.
func TestRollbackRejectsForeignSnapshot(t *testing.T) {
	store := &mismatchStore{snap: db.Snapshot{
		ID: "s1", NodeID: "nodeA", ContainerName: "plex",
		SpecJSON: `{"name":"plex","config":{}}`,
	}}
	nilProvider := func(context.Context, string) (*docker.Client, error) {
		t.Fatal("provider must not be called when the snapshot doesn't match")
		return nil, nil
	}
	s := New(store, nilProvider)

	_, err := s.Rollback(context.Background(), "s1", "nodeB", "plex") // wrong node
	if err != ErrSnapshotMismatch {
		t.Errorf("cross-node rollback = %v, want ErrSnapshotMismatch", err)
	}
	_, err = s.Rollback(context.Background(), "s1", "nodeA", "jellyfin") // wrong name
	if err != ErrSnapshotMismatch {
		t.Errorf("cross-name rollback = %v, want ErrSnapshotMismatch", err)
	}
}

func TestPinnedRef(t *testing.T) {
	const dg = "sha256:abc123"
	cases := []struct {
		ref, digest, want string
	}{
		{"nginx:latest", dg, "nginx@" + dg},
		{"nginx", dg, "nginx@" + dg},
		{"jellyfin/jellyfin:10.9", dg, "jellyfin/jellyfin@" + dg},
		{"ghcr.io:443/org/app:v1", dg, "ghcr.io:443/org/app@" + dg},
		{"ghcr.io/org/app", dg, "ghcr.io/org/app@" + dg},
		{"nginx@" + dg, dg, "nginx@" + dg}, // already pinned: unchanged
		{"", dg, ""},                       // no ref -> can't pin
		{"nginx:latest", "", ""},           // no digest -> can't pin
	}
	for _, c := range cases {
		if got := pinnedRef(c.ref, c.digest); got != c.want {
			t.Errorf("pinnedRef(%q, %q) = %q, want %q", c.ref, c.digest, got, c.want)
		}
	}
}

func TestRepoOnly(t *testing.T) {
	cases := map[string]string{
		"nginx:latest":            "nginx",
		"nginx":                   "nginx",
		"jellyfin/jellyfin:10.9":  "jellyfin/jellyfin",
		"ghcr.io:443/org/app:v1":  "ghcr.io:443/org/app",
		"ghcr.io:443/org/app":     "ghcr.io:443/org/app",
		"registry.local:5000/img": "registry.local:5000/img",
	}
	for in, want := range cases {
		if got := repoOnly(in); got != want {
			t.Errorf("repoOnly(%q) = %q, want %q", in, got, want)
		}
	}
}
