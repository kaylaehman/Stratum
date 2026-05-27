package sqlite_test

import (
	"context"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func seedNode(t *testing.T, st interface {
	CreateNode(context.Context, appdb.Node) error
}, id string) {
	t.Helper()
	if err := st.CreateNode(context.Background(), appdb.Node{
		ID: id, Name: "n", Type: "proxmox", Host: "h", Port: 22,
		AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1}, Status: "ok",
	}); err != nil {
		t.Fatalf("seed node: %v", err)
	}
}

func TestVMUpsertListDelete(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	seedNode(t, st, "node1")

	v := appdb.VM{ID: "vm1", NodeID: "node1", Kind: "qemu", ProxmoxVMID: 100, ProxmoxNode: "pve", Name: "ubuntu", Status: "running"}
	if err := st.UpsertVM(ctx, v); err != nil {
		t.Fatalf("UpsertVM insert: %v", err)
	}

	// Upsert again with a different ID but same natural key -> updates in place.
	v2 := v
	v2.ID = "different-uuid"
	v2.Status = "stopped"
	v2.Name = "ubuntu-renamed"
	if err := st.UpsertVM(ctx, v2); err != nil {
		t.Fatalf("UpsertVM conflict: %v", err)
	}

	list, err := st.ListVMsByNode(ctx, "node1")
	if err != nil {
		t.Fatalf("ListVMsByNode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1 (upsert should not duplicate)", len(list))
	}
	if list[0].ID != "vm1" {
		t.Errorf("id changed on conflict: got %q, want vm1 (original kept)", list[0].ID)
	}
	if list[0].Status != "stopped" || list[0].Name != "ubuntu-renamed" {
		t.Errorf("update not applied: %+v", list[0])
	}

	if err := st.DeleteVM(ctx, "vm1"); err != nil {
		t.Fatalf("DeleteVM: %v", err)
	}
	if list, _ := st.ListVMsByNode(ctx, "node1"); len(list) != 0 {
		t.Errorf("vm not deleted: %d rows", len(list))
	}
}

func TestContainerUpsertGoneSinceRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	seedNode(t, st, "node1")

	c := appdb.Container{ID: "c1", NodeID: "node1", DockerID: "abc123", Name: "plex", Image: "plex:latest", ImageID: "sha256:deadbeef", Status: "running", ComposeProject: "media"}
	if err := st.UpsertContainer(ctx, c); err != nil {
		t.Fatalf("UpsertContainer: %v", err)
	}

	// Mark gone_since.
	gone := time.Now()
	c.GoneSince = &gone
	c.Stale = true
	if err := st.UpsertContainer(ctx, c); err != nil {
		t.Fatalf("UpsertContainer update: %v", err)
	}

	list, _ := st.ListContainersByNode(ctx, "node1")
	if len(list) != 1 {
		t.Fatalf("len = %d", len(list))
	}
	got := list[0]
	if got.GoneSince == nil {
		t.Error("GoneSince not persisted")
	}
	if !got.Stale {
		t.Error("Stale not persisted")
	}
	if got.ImageID != "sha256:deadbeef" || got.ComposeProject != "media" {
		t.Errorf("fields lost: %+v", got)
	}
}
