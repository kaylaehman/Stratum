package sqlite_test

import (
	"context"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func sampleNode(id string) appdb.Node {
	return appdb.Node{
		ID:                   id,
		Name:                 "homelab-01",
		Type:                 "standalone",
		Host:                 "192.168.1.10",
		Port:                 22,
		AuthMethod:           "ssh_key",
		OSType:               "debian",
		CapabilitiesJSON:     `{"docker":true,"systemd":true,"cron":true,"proxmox_auth_status":"none"}`,
		CredentialsEncrypted: []byte{0x01, 0x02, 0x03, 0x04},
		CredentialsVersion:   1,
		SSHHostKey:           "ssh-ed25519 AAAAC3Nz...",
		Status:               "ok",
	}
}

func TestNodeCRUD(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if nodes, err := st.ListNodes(ctx); err != nil || len(nodes) != 0 {
		t.Fatalf("ListNodes initial = %v, %v; want empty", nodes, err)
	}

	n := sampleNode("n1")
	if err := st.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	got, err := st.GetNode(ctx, "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Name != "homelab-01" || got.Type != "standalone" || got.Port != 22 {
		t.Errorf("got %+v", got)
	}
	if string(got.CredentialsEncrypted) != string(n.CredentialsEncrypted) {
		t.Errorf("credentials blob round-trip mismatch")
	}
	if got.ProxmoxTLSInsecure {
		t.Error("ProxmoxTLSInsecure should default false")
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Error("timestamps not populated")
	}

	// Update
	got.Status = "unreachable"
	got.LastError = "ssh_unreachable"
	ls := time.Now()
	got.LastSeen = &ls
	got.ProxmoxTLSInsecure = true
	if err := st.UpdateNode(ctx, got); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	updated, _ := st.GetNode(ctx, "n1")
	if updated.Status != "unreachable" || updated.LastError != "ssh_unreachable" {
		t.Errorf("update not persisted: %+v", updated)
	}
	if updated.LastSeen == nil {
		t.Error("LastSeen not persisted")
	}
	if !updated.ProxmoxTLSInsecure {
		t.Error("ProxmoxTLSInsecure update not persisted")
	}

	// List
	if nodes, _ := st.ListNodes(ctx); len(nodes) != 1 {
		t.Errorf("ListNodes len = %d, want 1", len(nodes))
	}

	// Delete
	if err := st.DeleteNode(ctx, "n1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if _, err := st.GetNode(ctx, "n1"); err != appdb.ErrNotFound {
		t.Errorf("GetNode after delete = %v, want ErrNotFound", err)
	}
	if err := st.DeleteNode(ctx, "n1"); err != appdb.ErrNotFound {
		t.Errorf("DeleteNode missing = %v, want ErrNotFound", err)
	}
}
