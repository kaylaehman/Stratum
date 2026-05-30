package proxmox_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kaylaehman/stratum/backend/proxmox"
)

const (
	testTokenID = "root@pam!mytoken"
	testSecret  = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

// wantAuthHeader is the exact Authorization header value the client must send.
const wantAuthHeader = "PVEAPIToken=" + testTokenID + "=" + testSecret

// TestVersion_200 verifies a successful Proxmox version probe.
func TestVersion_200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert the correct Authorization header was sent.
		got := r.Header.Get("Authorization")
		if got != wantAuthHeader {
			t.Errorf("Authorization header = %q; want %q", got, wantAuthHeader)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"version":"8.2.4","release":"4","repoid":"abc123"}}`))
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	version, status, err := c.Version(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != proxmox.AuthConfirmed {
		t.Errorf("status = %q; want %q", status, proxmox.AuthConfirmed)
	}
	if version != "8.2.4" {
		t.Errorf("version = %q; want %q", version, "8.2.4")
	}
}

// TestVersion_401 verifies that a 401 response is treated as AuthUnauthed with
// no error — the endpoint IS Proxmox, the token is just wrong/missing.
func TestVersion_401(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	version, status, err := c.Version(context.Background())

	if err != nil {
		t.Fatalf("expected nil error on 401, got: %v", err)
	}
	if status != proxmox.AuthUnauthed {
		t.Errorf("status = %q; want %q", status, proxmox.AuthUnauthed)
	}
	if version != "" {
		t.Errorf("version = %q; want empty string", version)
	}
}

// TestVersion_500 verifies that an unexpected HTTP status produces an error.
func TestVersion_500(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	_, _, err := c.Version(context.Background())

	if err == nil {
		t.Fatal("expected an error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error message %q does not mention status 500", err.Error())
	}
}

// TestNodes verifies Nodes() parses online/offline members and both forms of
// the online indicator (numeric "online" field and string "status" field).
func TestNodes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Three members:
		//  - pve1: online via numeric field (online:1)
		//  - pve2: offline (online:0, status:"offline")
		//  - pve3: online via status string only (online:0, status:"online")
		_, _ = w.Write([]byte(`{"data":[
			{"node":"pve1","status":"online","online":1},
			{"node":"pve2","status":"offline","online":0},
			{"node":"pve3","status":"online","online":0}
		]}`))
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	nodes, err := c.Nodes(context.Background())
	if err != nil {
		t.Fatalf("Nodes() error: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("len(nodes) = %d; want 3", len(nodes))
	}

	cases := []struct {
		name   string
		online bool
	}{
		{"pve1", true},
		{"pve2", false},
		{"pve3", true},
	}
	for i, tc := range cases {
		if nodes[i].Name != tc.name {
			t.Errorf("nodes[%d].Name = %q; want %q", i, nodes[i].Name, tc.name)
		}
		if nodes[i].Online != tc.online {
			t.Errorf("nodes[%d].Online = %v; want %v", i, nodes[i].Online, tc.online)
		}
	}
}

// TestLocalNodeName verifies LocalNodeName() picks the type=="node" entry with
// local==1 from a clustered /cluster/status response (ignoring the "cluster"
// entry and the non-local members) and sends the correct auth header to the
// right path.
func TestLocalNodeName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if got != wantAuthHeader {
			t.Errorf("Authorization header = %q; want %q", got, wantAuthHeader)
		}
		if r.URL.Path != "/api2/json/cluster/status" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// A clustered host: a "cluster" entry plus three "node" members, with
		// proxmox2 flagged local==1 (this is the member that answered).
		_, _ = w.Write([]byte(`{"data":[
			{"type":"cluster","name":"homelab","local":0},
			{"type":"node","name":"proxmox1","local":0},
			{"type":"node","name":"proxmox2","local":1},
			{"type":"node","name":"proxmox3","local":0}
		]}`))
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	name, err := c.LocalNodeName(context.Background())
	if err != nil {
		t.Fatalf("LocalNodeName() error: %v", err)
	}
	if name != "proxmox2" {
		t.Errorf("LocalNodeName() = %q; want %q", name, "proxmox2")
	}
}

// TestLocalNodeName_Standalone verifies that a single (non-clustered) PVE host,
// whose cluster/status lists exactly one node flagged local==1, resolves to that
// node.
func TestLocalNodeName_Standalone(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[
			{"type":"node","name":"pve","local":1}
		]}`))
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	name, err := c.LocalNodeName(context.Background())
	if err != nil {
		t.Fatalf("LocalNodeName() error: %v", err)
	}
	if name != "pve" {
		t.Errorf("LocalNodeName() = %q; want %q", name, "pve")
	}
}

// TestLocalNodeName_NoLocal verifies that a response with no local==1 node entry
// returns an error (so the caller can fall back to enumerating all members).
func TestLocalNodeName_NoLocal(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// No entry has local==1 (e.g. unexpected API shape / permission gap).
		_, _ = w.Write([]byte(`{"data":[
			{"type":"cluster","name":"homelab","local":0},
			{"type":"node","name":"proxmox1","local":0}
		]}`))
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	_, err := c.LocalNodeName(context.Background())
	if err == nil {
		t.Fatal("expected an error when no local node is present, got nil")
	}
}

// TestLocalNodeName_403 verifies a non-200 status from cluster/status surfaces
// as an error.
func TestLocalNodeName_403(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	if _, err := c.LocalNodeName(context.Background()); err == nil {
		t.Fatal("expected error for HTTP 403, got nil")
	}
}

// TestQemuList verifies QemuList() parses VM data and sends the correct auth header.
func TestQemuList(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert the correct Authorization header.
		got := r.Header.Get("Authorization")
		if got != wantAuthHeader {
			t.Errorf("Authorization header = %q; want %q", got, wantAuthHeader)
		}
		if r.URL.Path != "/api2/json/nodes/pve/qemu" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[
			{"vmid":100,"name":"ubuntu","status":"running"},
			{"vmid":101,"name":"windows","status":"stopped"}
		]}`))
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	guests, err := c.QemuList(context.Background(), "pve")
	if err != nil {
		t.Fatalf("QemuList() error: %v", err)
	}
	if len(guests) != 2 {
		t.Fatalf("len(guests) = %d; want 2", len(guests))
	}

	expected := []proxmox.Guest{
		{VMID: 100, Name: "ubuntu", Status: "running", Kind: "qemu"},
		{VMID: 101, Name: "windows", Status: "stopped", Kind: "qemu"},
	}
	for i, want := range expected {
		got := guests[i]
		if got.VMID != want.VMID || got.Name != want.Name || got.Status != want.Status || got.Kind != want.Kind {
			t.Errorf("guests[%d] = %+v; want %+v", i, got, want)
		}
	}
}

// TestLxcList_StringVMID verifies LxcList() parses a vmid expressed as a quoted
// JSON string (as produced by some older PVE versions/endpoints).
func TestLxcList_StringVMID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve/lxc" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// vmid is a quoted string — the tricky case.
		_, _ = w.Write([]byte(`{"data":[
			{"vmid":"105","name":"alpine","status":"running"}
		]}`))
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	guests, err := c.LxcList(context.Background(), "pve")
	if err != nil {
		t.Fatalf("LxcList() error: %v", err)
	}
	if len(guests) != 1 {
		t.Fatalf("len(guests) = %d; want 1", len(guests))
	}
	g := guests[0]
	if g.VMID != 105 {
		t.Errorf("VMID = %d; want 105", g.VMID)
	}
	if g.Kind != "lxc" {
		t.Errorf("Kind = %q; want %q", g.Kind, "lxc")
	}
	if g.Name != "alpine" {
		t.Errorf("Name = %q; want %q", g.Name, "alpine")
	}
	if g.Status != "running" {
		t.Errorf("Status = %q; want %q", g.Status, "running")
	}
}

// TestNonOK_ReturnsError verifies that a non-200 status from the API returns an error.
func TestNonOK_ReturnsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)

	t.Run("nodes_403", func(t *testing.T) {
		_, err := c.Nodes(context.Background())
		if err == nil {
			t.Fatal("expected error for HTTP 403, got nil")
		}
	})

	t.Run("qemu_403", func(t *testing.T) {
		_, err := c.QemuList(context.Background(), "pve")
		if err == nil {
			t.Fatal("expected error for HTTP 403, got nil")
		}
	})

	t.Run("lxc_403", func(t *testing.T) {
		_, err := c.LxcList(context.Background(), "pve")
		if err == nil {
			t.Fatal("expected error for HTTP 403, got nil")
		}
	})
}

// TestVersion_TLS verifies the insecureSkipVerify toggle using httptest.NewTLSServer
// (which always presents a self-signed certificate).
func TestVersion_TLS(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"version":"8.1.0","release":"3","repoid":"xyz"}}`))
	}))
	defer srv.Close()

	// Subtests are sequential (no t.Parallel) so the deferred srv.Close() does
	// not fire until both subtests complete.

	// With insecureSkipVerify=false the TLS handshake must fail (cert not trusted).
	t.Run("strict_tls_fails", func(t *testing.T) {
		c := proxmox.New(srv.URL, testTokenID, testSecret, false)
		_, _, err := c.Version(context.Background())
		if err == nil {
			t.Fatal("expected TLS error with insecureSkipVerify=false, got nil")
		}
	})

	// With insecureSkipVerify=true the self-signed cert is accepted and the
	// probe succeeds.
	t.Run("skip_verify_succeeds", func(t *testing.T) {
		c := proxmox.New(srv.URL, testTokenID, testSecret, true)
		version, status, err := c.Version(context.Background())
		if err != nil {
			t.Fatalf("unexpected error with insecureSkipVerify=true: %v", err)
		}
		if status != proxmox.AuthConfirmed {
			t.Errorf("status = %q; want %q", status, proxmox.AuthConfirmed)
		}
		if version != "8.1.0" {
			t.Errorf("version = %q; want %q", version, "8.1.0")
		}
	})
}
