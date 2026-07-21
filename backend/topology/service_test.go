package topology

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	appdb "github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/db/sqlite"
	"github.com/KAE-Labs/stratum/backend/docker"
	"github.com/KAE-Labs/stratum/backend/nodeconn"
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

// fakeLister is a networkLister that returns canned networks (or an error),
// letting the topology service be exercised without a live Docker daemon.
type fakeLister struct {
	networks []docker.NetworkInfo
	err      error
}

func (f fakeLister) ListNetworks(context.Context) ([]docker.NetworkInfo, error) {
	return f.networks, f.err
}

// withLister swaps the service's network-lister seam so a test can inject a
// working or failing fake.
func withLister(s *Service, l networkLister, err error) {
	s.listerFor = func(context.Context, string) (networkLister, error) {
		if err != nil {
			return nil, err
		}
		return l, nil
	}
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

// TestForNode_NetworksReturnedForStandalone proves the core Bug-1 fix: a
// standalone-style node with a WORKING docker client returns its networks
// (bridge + user-defined), with container membership annotated — not an empty
// topology.
func TestForNode_NetworksReturnedForStandalone(t *testing.T) {
	st := testStore(t, appdb.Node{
		ID: "n1", Name: "test", Type: "standalone", Host: "h", Port: 22,
		AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1}, Status: "ok",
	})
	// One container present in inventory so membership can be annotated.
	if err := st.UpsertContainer(context.Background(), appdb.Container{
		ID: "c-internal-1", NodeID: "n1", DockerID: "deadbeefcafe0001",
		Name: "web", Status: "running",
	}); err != nil {
		t.Fatalf("UpsertContainer: %v", err)
	}

	nets := []docker.NetworkInfo{
		{ID: "net-bridge", Name: "bridge", Driver: "bridge"},
		{ID: "net-app", Name: "app-net", Driver: "bridge",
			Endpoints: []docker.NetworkEndpoint{{ContainerID: "deadbeefcafe0001", Name: "web"}}},
	}
	svc := New(st, providerFailing) // provider unused: lister seam overridden below
	withLister(svc, fakeLister{networks: nets}, nil)

	topo, err := svc.ForNode(context.Background(), "n1")
	if err != nil {
		t.Fatalf("ForNode returned unexpected error: %v", err)
	}
	if topo.DockerError != "" {
		t.Errorf("docker_error = %q, want empty (working client)", topo.DockerError)
	}
	if len(topo.Networks) != 2 {
		t.Fatalf("got %d networks, want 2 (bridge + app-net)", len(topo.Networks))
	}
	// Sorted by name: app-net, bridge.
	if topo.Networks[0].Name != "app-net" || topo.Networks[1].Name != "bridge" {
		t.Errorf("network order = [%s %s], want [app-net bridge]",
			topo.Networks[0].Name, topo.Networks[1].Name)
	}
	if len(topo.Containers) != 1 {
		t.Fatalf("got %d containers, want 1", len(topo.Containers))
	}
	web := topo.Containers[0]
	if len(web.Networks) != 1 || web.Networks[0] != "app-net" {
		t.Errorf("web networks = %v, want [app-net]", web.Networks)
	}
	if web.Isolated {
		t.Error("web should not be isolated: it is on app-net")
	}
}

// TestForNode_TransportErrorPropagates proves a stale-connection transport
// error from ListNetworks is RETURNED (so the API layer rebuilds + retries),
// not swallowed into DockerError — the exact reason networks came back empty
// while the poller still showed containers.
func TestForNode_TransportErrorPropagates(t *testing.T) {
	st := testStore(t, appdb.Node{
		ID: "n1", Name: "test", Type: "standalone", Host: "h", Port: 22,
		AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1}, Status: "ok",
	})
	svc := New(st, providerFailing)
	withLister(svc, fakeLister{err: io.EOF}, nil) // EOF => transport error

	_, err := svc.ForNode(context.Background(), "n1")
	if err == nil {
		t.Fatal("ForNode should propagate a transport error so the API can retry")
	}
	if !nodeconn.IsTransportError(err) {
		t.Errorf("propagated error is not a transport error: %v", err)
	}
}

// TestForNode_AppErrorYieldsDockerErrorNotPanic proves a non-transport
// ListNetworks failure degrades to an empty topology with DockerError set,
// never a panic or a propagated Go error.
func TestForNode_AppErrorYieldsDockerErrorNotPanic(t *testing.T) {
	st := testStore(t, appdb.Node{
		ID: "n1", Name: "test", Type: "standalone", Host: "h", Port: 22,
		AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1}, Status: "ok",
	})
	svc := New(st, providerFailing)
	withLister(svc, fakeLister{err: errors.New("permission denied")}, nil)

	topo, err := svc.ForNode(context.Background(), "n1")
	if err != nil {
		t.Fatalf("ForNode should not propagate a non-transport error: %v", err)
	}
	if topo.DockerError != "docker_list_networks_failed" {
		t.Errorf("docker_error = %q, want docker_list_networks_failed", topo.DockerError)
	}
	if len(topo.Networks) != 0 {
		t.Errorf("networks should be empty on app error, got %d", len(topo.Networks))
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
