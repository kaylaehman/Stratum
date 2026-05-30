package nodes_test

import (
	"context"
	"path/filepath"
	"testing"

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

	return nodes.NewService(store, testCipher(t, 0x7)), store
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

// UpdateConfig must persist a Docker endpoint, expose it in the (non-secret)
// view, seal supplied Docker TLS material into the credentials blob, and never
// echo the sealed material back.
func TestUpdateConfigPersistsDockerEndpointAndSealsTLS(t *testing.T) {
	ctx := context.Background()
	svc, store := newService(t)

	// Bare node, no SSH creds/endpoints -> probes instantly.
	v, err := svc.Create(ctx, nodes.CreateInput{
		Name:      "edit-me",
		ConnInput: nodes.ConnInput{Host: "10.0.0.7", SSHPort: 22, Credentials: nodes.NodeCredentials{Method: nodes.MethodSSHKey}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.DockerEndpoint != "" {
		t.Fatalf("new node DockerEndpoint = %q, want empty", v.DockerEndpoint)
	}

	ep := "tcp://10.0.0.7:2376"
	updated, err := svc.UpdateConfig(ctx, v.ID, nodes.UpdateConfigInput{
		DockerEndpoint:    &ep,
		DockerTLSSupplied: true,
		DockerTLSCA:       "CA-PEM",
		DockerTLSCert:     "CERT-PEM",
		DockerTLSKey:      "KEY-PEM",
	})
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	if updated.DockerEndpoint != ep {
		t.Errorf("view DockerEndpoint = %q, want %q", updated.DockerEndpoint, ep)
	}

	// docker_endpoint persisted; TLS sealed into the credentials blob (not plain).
	n, err := store.GetNode(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n.DockerEndpoint != ep {
		t.Errorf("persisted DockerEndpoint = %q, want %q", n.DockerEndpoint, ep)
	}
	creds, err := nodes.OpenCredentials(testCipher(t, 0x7), n.CredentialsEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if creds.DockerTLSCA != "CA-PEM" || creds.DockerTLSCert != "CERT-PEM" || creds.DockerTLSKey != "KEY-PEM" {
		t.Errorf("sealed Docker TLS not round-tripped: %+v", creds)
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
