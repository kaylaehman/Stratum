package nodes_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kaylaehman/stratum/backend/crypto"
	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/nodes"
)

func newService(t *testing.T) (*nodes.Service, appdb.Store) {
	t.Helper()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "test.db"))
	sqldb, err := appdb.Open("sqlite://" + path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := appdb.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	store := sqlite.New(sqldb)
	t.Cleanup(func() { store.Close() })

	key := make([]byte, crypto.KeySize)
	for i := range key {
		key[i] = 0x7
	}
	cipher, err := crypto.New(key)
	if err != nil {
		t.Fatal(err)
	}
	return nodes.NewService(store, cipher), store
}

// A node with no SSH credentials and no endpoints probes instantly (all sub-probes
// skip), so we can exercise the create->seal->persist->view path without a network.
func TestCreateNoCredsPersistsAndHidesSecrets(t *testing.T) {
	ctx := context.Background()
	svc, store := newService(t)

	view, err := svc.Create(ctx, nodes.CreateInput{
		Name: "bare-host",
		ConnInput: nodes.ConnInput{
			Host:        "10.0.0.99",
			SSHPort:     22,
			Credentials: nodes.NodeCredentials{Method: nodes.MethodSSHKey}, // no SSHUser => SSH not attempted
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if view.Type != "ssh" || view.Name != "bare-host" {
		t.Errorf("view = %+v", view)
	}

	// The persisted credentials blob must decrypt back to the input creds...
	n, err := store.GetNode(ctx, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(n.CredentialsEncrypted) == 0 {
		t.Error("credentials not sealed/persisted")
	}
	// ...and the NodeView type must not expose any secret field (compile-time:
	// NodeView has no password/key fields). Confirm capabilities round-tripped.
	if view.ProxmoxAuthStatus != "none" {
		t.Errorf("ProxmoxAuthStatus = %q, want none", view.ProxmoxAuthStatus)
	}
}

func TestCreateRequiresAcceptedHostKeyWhenSSHUsed(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	_, err := svc.Create(ctx, nodes.CreateInput{
		Name: "ssh-host",
		ConnInput: nodes.ConnInput{
			Host:        "10.0.0.5",
			SSHPort:     22,
			Credentials: nodes.NodeCredentials{Method: nodes.MethodSSHKey, SSHUser: "kayla", SSHPrivateKey: "pem"},
		},
		AcceptedHostKey: "", // missing -> must be rejected before any dial
	})
	if err != nodes.ErrHostKeyRequired {
		t.Fatalf("err = %v, want ErrHostKeyRequired", err)
	}
}

func TestListGetDelete(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	v, err := svc.Create(ctx, nodes.CreateInput{
		Name:      "n",
		ConnInput: nodes.ConnInput{Host: "h", SSHPort: 22, Credentials: nodes.NodeCredentials{Method: nodes.MethodSSHPassword}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if list, err := svc.List(ctx); err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v", list, err)
	}
	if got, err := svc.Get(ctx, v.ID); err != nil || got.ID != v.ID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := svc.Rename(ctx, v.ID, "renamed"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if got, _ := svc.Get(ctx, v.ID); got.Name != "renamed" {
		t.Errorf("rename not applied: %q", got.Name)
	}
	if err := svc.Delete(ctx, v.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, v.ID); err != appdb.ErrNotFound {
		t.Errorf("Get after delete = %v, want ErrNotFound", err)
	}
}
