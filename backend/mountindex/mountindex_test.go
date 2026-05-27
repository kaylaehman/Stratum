package mountindex

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/docker"
)

func setup(t *testing.T) (*Index, appdb.Store) {
	t.Helper()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "test.db"))
	sqldb, err := appdb.Open("sqlite://" + path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := appdb.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st := sqlite.New(sqldb)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	if err := st.CreateNode(ctx, appdb.Node{ID: "n1", Name: "n", Type: "standalone", Host: "h", Port: 22, AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1}, Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"c1", "c2", "c3"} {
		if err := st.UpsertContainer(ctx, appdb.Container{ID: id, NodeID: "n1", DockerID: "d-" + id, Name: id, Image: "img", Status: "running"}); err != nil {
			t.Fatal(err)
		}
	}
	// Provider errors — never called because we pre-seed.
	ix := New(st, func(context.Context, string) (*docker.Client, error) { return nil, context.Canceled }, time.Minute)
	ix.seeded["n1"] = time.Now() // skip ensureFresh
	return ix, st
}

func seedMounts(t *testing.T, st appdb.Store) {
	ctx := context.Background()
	// c1 and c2 both bind /data; c3 binds /data-archive. c1 and c2 also share volume "appdata".
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(st.ReplaceContainerMounts(ctx, "c1", []appdb.MountRow{
		{ID: "m1", NodeID: "n1", ContainerID: "c1", Type: "bind", Source: "/data", NormalizedSource: "/data", Destination: "/data", RW: true},
		{ID: "m2", NodeID: "n1", ContainerID: "c1", Type: "volume", Source: "/var/lib/docker/volumes/appdata/_data", NormalizedSource: "/var/lib/docker/volumes/appdata/_data", VolumeName: "appdata", Destination: "/app", RW: true},
	}))
	must(st.ReplaceContainerMounts(ctx, "c2", []appdb.MountRow{
		{ID: "m3", NodeID: "n1", ContainerID: "c2", Type: "bind", Source: "/data", NormalizedSource: "/data", Destination: "/d", RW: false},
		{ID: "m4", NodeID: "n1", ContainerID: "c2", Type: "volume", Source: "/var/lib/docker/volumes/appdata/_data", NormalizedSource: "/var/lib/docker/volumes/appdata/_data", VolumeName: "appdata", Destination: "/app", RW: true},
	}))
	must(st.ReplaceContainerMounts(ctx, "c3", []appdb.MountRow{
		{ID: "m5", NodeID: "n1", ContainerID: "c3", Type: "bind", Source: "/data-archive", NormalizedSource: "/data-archive", Destination: "/arch", RW: true},
	}))
}

func TestReverseParentAndSegmentAware(t *testing.T) {
	ix, st := setup(t)
	seedMounts(t, st)
	hits, err := ix.Reverse(context.Background(), "n1", "/data/app/logs")
	if err != nil {
		t.Fatal(err)
	}
	// c1 and c2 (parent /data) match; c3 (/data-archive) must NOT.
	ctrs := map[string]bool{}
	for _, h := range hits {
		ctrs[h.ContainerID] = true
		if h.Relation != docker.RelAParentB {
			t.Errorf("expected parent relation, got %q", h.Relation)
		}
	}
	if !ctrs["c1"] || !ctrs["c2"] || ctrs["c3"] {
		t.Fatalf("reverse hits = %v; want c1,c2 not c3", ctrs)
	}
}

func TestSharedBindAndVolume(t *testing.T) {
	ix, st := setup(t)
	seedMounts(t, st)
	shared, err := ix.Shared(context.Background(), "n1")
	if err != nil {
		t.Fatal(err)
	}
	var bindShared, volShared bool
	for _, s := range shared {
		if s.Kind == "bind" && s.Key == "/data" && len(s.ContainerIDs) == 2 {
			bindShared = true
		}
		if s.Kind == "volume" && s.Key == "appdata" && len(s.ContainerIDs) == 2 {
			volShared = true
		}
	}
	if !bindShared {
		t.Error("expected /data shared as a bind across 2 containers")
	}
	if !volShared {
		t.Error("expected appdata shared as a volume across 2 containers (by Name, not _data path)")
	}
}

func TestForwardSharedFlag(t *testing.T) {
	ix, st := setup(t)
	seedMounts(t, st)
	views, err := ix.Forward(context.Background(), "n1", "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 {
		t.Fatalf("c1 should have 2 mounts, got %d", len(views))
	}
	for _, v := range views {
		if !v.Shared {
			t.Errorf("c1's mount %s should be flagged shared", v.Destination)
		}
		if !v.Traceable {
			t.Errorf("bind/volume should be traceable: %+v", v)
		}
	}
}
