package depgraph

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/mountindex"
	"github.com/kaylaehman/stratum/backend/nodeconn"
)

func TestMatchInternal(t *testing.T) {
	idx := map[string]string{
		"abcdef0123456789aaaa": "c1", // full docker id -> internal id
		"112233445566":         "c2", // already-short (12-char) docker id
	}
	cases := []struct {
		docker string
		want   string
	}{
		{"abcdef0123456789aaaa", "c1"}, // exact full
		{"abcdef012345", "c1"},         // short prefix of a full id
		{"112233445566", "c2"},         // exact short
		{"112233445566ffff", "c2"},     // full extends a stored short id
		{"deadbeef0000", ""},           // no match
		{"short", ""},                  // <12 chars, no exact -> no fuzzy match
	}
	for _, c := range cases {
		if got := matchInternal(idx, c.docker); got != c.want {
			t.Errorf("matchInternal(%q) = %q, want %q", c.docker, got, c.want)
		}
	}
}

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

// failingMountProvider always errors, so the mount index's EnsureFresh seeds no
// rows and volumeEdges degrades to no volume edges — keeping these tests focused
// on the network/reachability paths without a live daemon.
func failingMountProvider(_ context.Context, _ string) (*docker.Client, error) {
	return nil, errors.New("no docker client")
}

func testMountIndex(t *testing.T, st appdb.Store) *mountindex.Index {
	t.Helper()
	return mountindex.New(st, failingMountProvider, 30*time.Second)
}

// fakeLister is a networkLister returning canned networks (or an error).
type fakeLister struct {
	networks []docker.NetworkInfo
	err      error
}

func (f fakeLister) ListNetworks(context.Context) ([]docker.NetworkInfo, error) {
	return f.networks, f.err
}

// withLister swaps the depgraph service's lister seam to inject a fake.
func withLister(s *Service, l networkLister, provErr error) {
	s.listerFor = func(context.Context, string) (networkLister, error) {
		if provErr != nil {
			return nil, provErr
		}
		return l, nil
	}
}

func newNode(id, typ, status string) appdb.Node {
	return appdb.Node{
		ID: id, Name: id, Type: typ, Host: "h", Port: 22,
		AuthMethod: "ssh_key", CredentialsEncrypted: []byte{1}, Status: status,
		// Docker capability present; this is the gate the API checks.
		CapabilitiesJSON: `{"docker":true}`,
	}
}

// ── ForNode: reachability + node-set composition ─────────────────────────────

// TestForNode_ReachableStandaloneReturnsGraph proves a standalone docker node
// that's up assembles a graph with network nodes/edges — and crucially does NOT
// surface as unreachable.
func TestForNode_ReachableStandaloneReturnsGraph(t *testing.T) {
	st := testStore(t, newNode("n1", "standalone", "ok"))
	if err := st.UpsertContainer(context.Background(), appdb.Container{
		ID: "ci1", NodeID: "n1", DockerID: "deadbeefcafe0001", Name: "web", Status: "running",
	}); err != nil {
		t.Fatalf("UpsertContainer: %v", err)
	}
	nets := []docker.NetworkInfo{
		{ID: "nb", Name: "bridge", Driver: "bridge"},
		{ID: "na", Name: "app-net", Driver: "bridge",
			Endpoints: []docker.NetworkEndpoint{{ContainerID: "deadbeefcafe0001"}}},
	}
	svc := New(st, failingMountProvider, testMountIndex(t, st))
	withLister(svc, fakeLister{networks: nets}, nil)

	g, err := svc.ForNode(context.Background(), "n1")
	if err != nil {
		t.Fatalf("ForNode returned error for a reachable node: %v", err)
	}
	if errors.Is(err, ErrNoDockerClient) {
		t.Fatal("reachable node must not yield ErrNoDockerClient")
	}
	var gotNet, gotContainer int
	for _, n := range g.Nodes {
		switch n.Kind {
		case KindNetwork:
			gotNet++
		case KindContainer:
			gotContainer++
		}
	}
	if gotNet != 2 {
		t.Errorf("network nodes = %d, want 2", gotNet)
	}
	if gotContainer != 1 {
		t.Errorf("container nodes = %d, want 1", gotContainer)
	}
	// One membership edge: web -> app-net.
	var memberEdges int
	for _, e := range g.Edges {
		if e.Kind == KindNetwork {
			memberEdges++
		}
	}
	if memberEdges != 1 {
		t.Errorf("network membership edges = %d, want 1", memberEdges)
	}
}

// TestForNode_NoDockerClientIsNotUnreachable proves that a node with the docker
// capability but no usable client (e.g. Docker inferred over SSH, no endpoint)
// yields ErrNoDockerClient — distinct from a genuine transport failure — so the
// API maps it to docker_not_available, not node_unreachable.
func TestForNode_NoDockerClientIsNotUnreachable(t *testing.T) {
	st := testStore(t, newNode("n1", "standalone", "ok"))
	svc := New(st, failingMountProvider, testMountIndex(t, st))
	withLister(svc, nil, errors.New("node n1 has no docker client"))

	_, err := svc.ForNode(context.Background(), "n1")
	if !errors.Is(err, ErrNoDockerClient) {
		t.Fatalf("err = %v, want ErrNoDockerClient", err)
	}
	if nodeconn.IsTransportError(err) {
		t.Error("no-docker-client must NOT be classified as a transport error")
	}
}

// TestForNode_TransportErrorPropagates proves a stale-connection transport error
// from ListNetworks is returned (so the API rebuilds + retries), not swallowed.
func TestForNode_TransportErrorPropagates(t *testing.T) {
	st := testStore(t, newNode("n1", "standalone", "ok"))
	svc := New(st, failingMountProvider, testMountIndex(t, st))
	withLister(svc, fakeLister{err: io.EOF}, nil)

	_, err := svc.ForNode(context.Background(), "n1")
	if err == nil {
		t.Fatal("transport error from ListNetworks must propagate for the API retry")
	}
	if !nodeconn.IsTransportError(err) {
		t.Errorf("propagated error is not a transport error: %v", err)
	}
	if errors.Is(err, ErrNoDockerClient) {
		t.Error("transport failure must not be reported as ErrNoDockerClient")
	}
}

// TestForNode_AppNetworkErrorDegradesGracefully proves a non-transport
// ListNetworks failure drops network nodes/edges but still returns the graph
// (containers render), never a panic or propagated error.
func TestForNode_AppNetworkErrorDegradesGracefully(t *testing.T) {
	st := testStore(t, newNode("n1", "standalone", "ok"))
	if err := st.UpsertContainer(context.Background(), appdb.Container{
		ID: "ci1", NodeID: "n1", DockerID: "deadbeefcafe0001", Name: "web", Status: "running",
	}); err != nil {
		t.Fatalf("UpsertContainer: %v", err)
	}
	svc := New(st, failingMountProvider, testMountIndex(t, st))
	withLister(svc, fakeLister{err: errors.New("permission denied")}, nil)

	g, err := svc.ForNode(context.Background(), "n1")
	if err != nil {
		t.Fatalf("non-transport network error must not fail the request: %v", err)
	}
	for _, n := range g.Nodes {
		if n.Kind == KindNetwork {
			t.Errorf("expected no network nodes on app error, got %q", n.Label)
		}
	}
	// The container node still renders.
	var gotContainer int
	for _, n := range g.Nodes {
		if n.Kind == KindContainer {
			gotContainer++
		}
	}
	if gotContainer != 1 {
		t.Errorf("container nodes = %d, want 1", gotContainer)
	}
}

// TestForNode_ProxmoxWithDockerNotExcluded proves the backend depgraph path is
// type-agnostic: a proxmox-typed node that ALSO runs Docker assembles a graph
// just like a standalone node. (The dropdown's frontend filter is separate; the
// backend must not reject on node.type.)
func TestForNode_ProxmoxWithDockerNotExcluded(t *testing.T) {
	st := testStore(t, newNode("pve1", "proxmox", "ok"))
	if err := st.UpsertContainer(context.Background(), appdb.Container{
		ID: "ci1", NodeID: "pve1", DockerID: "deadbeefcafe0001", Name: "web", Status: "running",
	}); err != nil {
		t.Fatalf("UpsertContainer: %v", err)
	}
	nets := []docker.NetworkInfo{{ID: "nb", Name: "bridge", Driver: "bridge"}}
	svc := New(st, failingMountProvider, testMountIndex(t, st))
	withLister(svc, fakeLister{networks: nets}, nil)

	g, err := svc.ForNode(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("proxmox-with-docker node must assemble a graph, got error: %v", err)
	}
	if g.NodeID != "pve1" {
		t.Errorf("graph node id = %q, want pve1", g.NodeID)
	}
	var gotNet int
	for _, n := range g.Nodes {
		if n.Kind == KindNetwork {
			gotNet++
		}
	}
	if gotNet != 1 {
		t.Errorf("network nodes = %d, want 1 (bridge) for proxmox+docker node", gotNet)
	}
}
