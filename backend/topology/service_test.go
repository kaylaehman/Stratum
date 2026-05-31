package topology

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/docker"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func testStore(t *testing.T, nodes ...appdb.Node) appdb.Store {
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
	for _, n := range nodes {
		if err := st.CreateNode(context.Background(), n); err != nil {
			t.Fatalf("CreateNode %s: %v", n.ID, err)
		}
	}
	return st
}

var errProviderFail = errors.New("docker client unavailable")

// providerFailing always returns an error (no Docker endpoint configured).
func providerFailing(_ context.Context, _ string) (*docker.Client, error) {
	return nil, errProviderFail
}

// ── ForNode: node_status passthrough ─────────────────────────────────────────

// TestForNode_NodeStatusPassthrough verifies that ForNode always returns the
// DB-stored node status and never infers reachability from the Docker probe.
func TestForNode_NodeStatusPassthrough(t *testing.T) {
	cases := []struct {
		name           string
		nodeStatus     string
		wantNodeStatus string
	}{
		{"ok_node", "ok", "ok"},
		{"unreachable_node", "unreachable", "unreachable"},
		{"empty_status", "", "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := testStore(t, appdb.Node{
				ID: "n1", Name: "test", Type: "standalone", Host: "h", Port: 22,
				AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1},
				Status: tc.nodeStatus,
			})
			svc := New(st, providerFailing)
			topo, err := svc.ForNode(context.Background(), "n1")
			if err != nil {
				t.Fatalf("ForNode returned unexpected error: %v", err)
			}
			if topo.NodeStatus != tc.wantNodeStatus {
				t.Errorf("node_status = %q, want %q", topo.NodeStatus, tc.wantNodeStatus)
			}
		})
	}
}

// TestForNode_DockerErrorOnProviderFailure verifies that a Docker provider
// failure is recorded in DockerError, not propagated as a Go error.
func TestForNode_DockerErrorOnProviderFailure(t *testing.T) {
	st := testStore(t, appdb.Node{
		ID: "n1", Name: "test", Type: "standalone", Host: "h", Port: 22,
		AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1}, Status: "ok",
	})
	svc := New(st, providerFailing)
	topo, err := svc.ForNode(context.Background(), "n1")
	if err != nil {
		t.Fatalf("ForNode should not return a Go error on Docker failure, got: %v", err)
	}
	if topo.DockerError == "" {
		t.Error("docker_error should be non-empty when provider fails")
	}
	if len(topo.Networks) != 0 {
		t.Errorf("networks should be empty on provider failure, got %d", len(topo.Networks))
	}
	if len(topo.Containers) != 0 {
		t.Errorf("containers should be empty on provider failure, got %d", len(topo.Containers))
	}
}

// TestForNode_OkNodeWithDockerError proves that an "ok" node with Docker
// unavailable still comes back with node_status="ok", NOT "unreachable" — the
// poller's SSH-reachability result is authoritative.
func TestForNode_OkNodeWithDockerError(t *testing.T) {
	st := testStore(t, appdb.Node{
		ID: "n1", Name: "test", Type: "standalone", Host: "h", Port: 22,
		AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1}, Status: "ok",
	})
	svc := New(st, providerFailing)
	topo, err := svc.ForNode(context.Background(), "n1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// node_status must reflect the DB value (poller-set "ok"), not the Docker error.
	if topo.NodeStatus != "ok" {
		t.Errorf("node_status = %q; want %q (DB truth)", topo.NodeStatus, "ok")
	}
	if topo.DockerError == "" {
		t.Error("docker_error must be non-empty to explain why topology is empty")
	}
}

// ── buildContainerNodes ───────────────────────────────────────────────────────

func TestBuildContainerNodes(t *testing.T) {
	networks := []docker.NetworkInfo{
		{Name: "bridge", Endpoints: []docker.NetworkEndpoint{{ContainerID: "c1"}}},
		{Name: "backend", Endpoints: []docker.NetworkEndpoint{{ContainerID: "c1"}, {ContainerID: "c2"}}},
		{Name: "host", Endpoints: []docker.NetworkEndpoint{{ContainerID: "c3"}}},
	}
	containers := []appdb.Container{
		{DockerID: "c1", Name: "web", Status: "running"},
		{DockerID: "c2", Name: "db", Status: "running"},
		{DockerID: "c3", Name: "monitor", Status: "running"},
		{DockerID: "c4", Name: "lonely", Status: "exited"},
	}

	got := buildContainerNodes(networks, containers)
	byName := map[string]ContainerNode{}
	for _, c := range got {
		byName[c.Name] = c
	}

	// c1 is on two networks (sorted), not isolated, not host.
	web := byName["web"]
	if len(web.Networks) != 2 || web.Networks[0] != "backend" || web.Networks[1] != "bridge" {
		t.Errorf("web networks = %v, want [backend bridge]", web.Networks)
	}
	if web.Isolated || web.HostNetwork {
		t.Errorf("web should be neither isolated nor host: %+v", web)
	}
	// c3 is on the host network.
	if !byName["monitor"].HostNetwork {
		t.Error("monitor should be flagged host_network")
	}
	// c4 is in no network => isolated.
	if !byName["lonely"].Isolated || len(byName["lonely"].Networks) != 0 {
		t.Errorf("lonely should be isolated with no networks: %+v", byName["lonely"])
	}
	// Output sorted by name.
	if got[0].Name != "db" {
		t.Errorf("first (sorted) = %s, want db", got[0].Name)
	}
}
