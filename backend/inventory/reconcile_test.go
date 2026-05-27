package inventory

import (
	"context"
	"path/filepath"
	"testing"

	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
)

func testStore(t *testing.T) appdb.Store {
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
	if err := st.CreateNode(context.Background(), appdb.Node{
		ID: "n1", Name: "n", Type: "standalone", Host: "h", Port: 22,
		AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1}, Status: "ok",
	}); err != nil {
		t.Fatal(err)
	}
	return st
}

func opsByKind(deltas []Delta) map[Op]int {
	m := map[Op]int{}
	for _, d := range deltas {
		m[d.Op]++
	}
	return m
}

func TestReconcileContainers_AddUpdateNoChange(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)

	// First poll: one container -> added.
	d1, err := reconcileContainers(ctx, st, "n1", []appdb.Container{
		{NodeID: "n1", DockerID: "abc", Name: "plex", Image: "plex:1", Status: "running"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(d1) != 1 || d1[0].Op != OpAdded {
		t.Fatalf("expected 1 added, got %+v", d1)
	}

	// Second poll, same state -> NO delta.
	d2, _ := reconcileContainers(ctx, st, "n1", []appdb.Container{
		{NodeID: "n1", DockerID: "abc", Name: "plex", Image: "plex:1", Status: "running"},
	})
	if len(d2) != 0 {
		t.Fatalf("expected no delta on unchanged poll, got %+v", d2)
	}

	// Third poll, status changed -> updated.
	d3, _ := reconcileContainers(ctx, st, "n1", []appdb.Container{
		{NodeID: "n1", DockerID: "abc", Name: "plex", Image: "plex:1", Status: "exited"},
	})
	if len(d3) != 1 || d3[0].Op != OpUpdated || d3[0].Container.Status != "exited" {
		t.Fatalf("expected 1 updated to exited, got %+v", d3)
	}
}

func TestReconcileContainers_GoneGraceTwoPolls(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)

	reconcileContainers(ctx, st, "n1", []appdb.Container{{NodeID: "n1", DockerID: "abc", Name: "plex", Image: "i", Status: "running"}})

	// First absence -> stale update, NOT removed.
	d1, _ := reconcileContainers(ctx, st, "n1", []appdb.Container{})
	if len(d1) != 1 || d1[0].Op != OpUpdated || !d1[0].Container.Stale {
		t.Fatalf("first absence should be a stale update, got %+v", d1)
	}
	if list, _ := st.ListContainersByNode(ctx, "n1"); len(list) != 1 {
		t.Fatal("container should still exist after first absence")
	}

	// Second consecutive absence -> removed.
	d2, _ := reconcileContainers(ctx, st, "n1", []appdb.Container{})
	if len(d2) != 1 || d2[0].Op != OpRemoved {
		t.Fatalf("second absence should remove, got %+v", d2)
	}
	if list, _ := st.ListContainersByNode(ctx, "n1"); len(list) != 0 {
		t.Fatal("container should be deleted after second absence")
	}
}

func TestReconcileContainers_FlapWithinGraceSurvives(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	full := []appdb.Container{{NodeID: "n1", DockerID: "abc", Name: "plex", Image: "i", Status: "running"}}

	reconcileContainers(ctx, st, "n1", full)
	reconcileContainers(ctx, st, "n1", []appdb.Container{}) // first absence -> stale
	// Reappears before the second absent poll -> back to healthy, an update.
	d, _ := reconcileContainers(ctx, st, "n1", full)
	if len(d) != 1 || d[0].Op != OpUpdated || d[0].Container.Stale {
		t.Fatalf("reappearance should clear stale via update, got %+v", d)
	}
	if list, _ := st.ListContainersByNode(ctx, "n1"); len(list) != 1 || list[0].Stale || list[0].GoneSince != nil {
		t.Fatalf("row should be healthy again: %+v", list)
	}
}

func TestReconcileContainers_RecreateIsRemovePlusAdd(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	reconcileContainers(ctx, st, "n1", []appdb.Container{{NodeID: "n1", DockerID: "old", Name: "plex", Image: "i", Status: "running"}})
	reconcileContainers(ctx, st, "n1", []appdb.Container{}) // old: first absence -> stale

	// New container id appears while old is gone for its 2nd poll.
	d, _ := reconcileContainers(ctx, st, "n1", []appdb.Container{{NodeID: "n1", DockerID: "new", Name: "plex", Image: "i", Status: "running"}})
	ops := opsByKind(d)
	if ops[OpAdded] != 1 || ops[OpRemoved] != 1 {
		t.Fatalf("recreate should be 1 add + 1 remove, got %+v", d)
	}
}

func TestReconcileVMs_AddAndStatusUpdate(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	d1, _ := reconcileVMs(ctx, st, "n1", []appdb.VM{{NodeID: "n1", Kind: "qemu", ProxmoxVMID: 100, ProxmoxNode: "pve", Name: "u", Status: "running"}})
	if len(d1) != 1 || d1[0].Op != OpAdded || d1[0].VM.ProxmoxVMID != 100 {
		t.Fatalf("expected 1 add, got %+v", d1)
	}
	d2, _ := reconcileVMs(ctx, st, "n1", []appdb.VM{{NodeID: "n1", Kind: "qemu", ProxmoxVMID: 100, ProxmoxNode: "pve", Name: "u", Status: "stopped"}})
	if len(d2) != 1 || d2[0].Op != OpUpdated || d2[0].VM.Status != "stopped" {
		t.Fatalf("expected 1 status update, got %+v", d2)
	}
}
