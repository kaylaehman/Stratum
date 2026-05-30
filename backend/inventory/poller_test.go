package inventory

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/crypto"
	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/nodeconn"
	"github.com/kaylaehman/stratum/backend/nodes"
)

func TestAcquireSkipIfBusy(t *testing.T) {
	p := &Poller{inflight: map[string]bool{}}
	if !p.acquire("n1") {
		t.Fatal("first acquire should succeed")
	}
	if p.acquire("n1") {
		t.Error("second acquire should be skipped while in-flight")
	}
	// A different node is independent.
	if !p.acquire("n2") {
		t.Error("acquire for a different node should succeed")
	}
	p.release("n1")
	if !p.acquire("n1") {
		t.Error("acquire after release should succeed")
	}
}

func TestUpdateNodeStatus_RecordsReachabilityAndError(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	p := &Poller{store: st, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	n, err := st.GetNode(ctx, "n1")
	if err != nil {
		t.Fatal(err)
	}

	// Unreachable with a sanitized SSH error category → persisted (no more empty
	// last_error on a down node).
	p.updateNodeStatus(ctx, n, false, "ssh_detect_failed")
	n, _ = st.GetNode(ctx, "n1")
	if n.Status != "unreachable" || n.LastError != "ssh_detect_failed" {
		t.Fatalf("after unreachable: status=%q last_error=%q", n.Status, n.LastError)
	}

	// Reachable → ok, error cleared, last_seen stamped.
	p.updateNodeStatus(ctx, n, true, "")
	n, _ = st.GetNode(ctx, "n1")
	if n.Status != "ok" || n.LastError != "" || n.LastSeen == nil {
		t.Fatalf("after reachable: status=%q last_error=%q last_seen=%v", n.Status, n.LastError, n.LastSeen)
	}
}

// TestPollNode_DockerHealthySSHFailing_OkAndEnumerates proves plane-aware
// reachability: a node whose Docker endpoint is healthy but whose SSH probe
// fails is still marked "ok" AND its containers are enumerated. Docker
// enumeration must NOT be gated on the SSH/reachability result.
func TestPollNode_DockerHealthySSHFailing_OkAndEnumerates(t *testing.T) {
	ctx := context.Background()

	// Store with one standalone node that advertises Docker + a tcp endpoint,
	// and sealed (decryptable) credentials so nodeconn can build a Docker client.
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

	key := make([]byte, crypto.KeySize)
	cipher, err := crypto.New(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := nodes.NodeCredentials{Method: nodes.MethodSSHKey, SSHUser: "kayla"}.Seal(cipher)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateNode(ctx, appdb.Node{
		ID: "dock1", Name: "dock", Type: "standalone", Host: "10.0.0.50", Port: 22,
		AuthMethod: "ssh_key", CapabilitiesJSON: `{"docker":true}`,
		CredentialsEncrypted: sealed, DockerEndpoint: "tcp://10.0.0.50:2375",
		Status: "unknown",
	}); err != nil {
		t.Fatal(err)
	}

	p := NewPoller(st, nodeconn.NewManager(st, cipher), hub.New(),
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	// SSH plane is DOWN: the reachability fallback reports unreachable.
	p.SetReachability(func(context.Context, appdb.Node) (bool, string) {
		return false, "ssh_unreachable"
	})

	// Docker plane is UP: the injected enumerator returns one container.
	enumCalled := false
	p.enumContainers = func(ctx context.Context, _ containerLister, nodeID string) ([]Delta, error) {
		enumCalled = true
		return reconcileContainers(ctx, st, nodeID, []appdb.Container{
			{NodeID: nodeID, DockerID: "abc123", Name: "plex", Image: "plex:1", Status: "running"},
		})
	}

	node, err := st.GetNode(ctx, "dock1")
	if err != nil {
		t.Fatal(err)
	}
	p.pollNode(ctx, node)

	if !enumCalled {
		t.Fatal("expected Docker enumeration to run despite SSH failing")
	}
	n, _ := st.GetNode(ctx, "dock1")
	if n.Status != "ok" {
		t.Errorf("status = %q, want ok (Docker plane healthy)", n.Status)
	}
	if n.LastError != "" {
		t.Errorf("last_error = %q, want empty on ok node", n.LastError)
	}
	if n.LastSeen == nil {
		t.Error("last_seen not stamped on ok node")
	}
	cs, err := st.ListContainersByNode(ctx, "dock1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Name != "plex" {
		t.Fatalf("containers = %+v, want one (plex) enumerated from the Docker plane", cs)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	st := testStore(t) // creates one node "n1" with no reachable clients
	key := make([]byte, crypto.KeySize)
	cipher, _ := crypto.New(key)
	p := NewPoller(st, nodeconn.NewManager(st, cipher), hub.New(),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.interval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	time.Sleep(60 * time.Millisecond) // let a couple ticks run
	cancel()

	select {
	case <-done:
		// returned promptly after cancel
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}
