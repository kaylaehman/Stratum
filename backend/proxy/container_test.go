package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/KAE-Labs/stratum/backend/crypto"
	"github.com/KAE-Labs/stratum/backend/db"
)

// cpStore is a minimal db.Store for the container-proxy service tests. It embeds
// the interface so only the handful of methods the code path touches need
// implementing; any unexpected call panics, surfacing accidental dependencies.
type cpStore struct {
	db.Store
	containers map[string]db.Container       // by inventory id
	byNode     map[string][]db.Container     // node id → containers
	nodes      []db.Node
	ports      []db.PortExposureRow
	proxyCfg   map[string]db.ProxyConfig // node id → config
}

func (s *cpStore) GetContainer(_ context.Context, id string) (db.Container, error) {
	c, ok := s.containers[id]
	if !ok {
		return db.Container{}, db.ErrNotFound
	}
	return c, nil
}
func (s *cpStore) ListNodes(context.Context) ([]db.Node, error) { return s.nodes, nil }
func (s *cpStore) ListContainersByNode(_ context.Context, nodeID string) ([]db.Container, error) {
	return s.byNode[nodeID], nil
}
func (s *cpStore) ListAllPortExposures(context.Context) ([]db.PortExposureRow, error) {
	return s.ports, nil
}
func (s *cpStore) GetProxyConfig(_ context.Context, nodeID string) (db.ProxyConfig, error) {
	if c, ok := s.proxyCfg[nodeID]; ok {
		return c, nil
	}
	return db.ProxyConfig{NodeID: nodeID}, nil
}
func (s *cpStore) ListMountsByNode(context.Context, string) ([]db.MountRow, error) {
	return nil, nil
}

func testCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	key := make([]byte, crypto.KeySize)
	for i := range key {
		key[i] = byte(i + 1)
	}
	c, err := crypto.New(key)
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return c
}

func sealToken(t *testing.T, c *crypto.Cipher, tok string) []byte {
	t.Helper()
	b, err := c.Seal([]byte(tok))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	return b
}

func TestConfigured(t *testing.T) {
	s := &Service{}
	cf := &CloudflareAPI{}
	if s.configured(cf, Conn{}, false) {
		t.Error("cloudflare-api with no token must be unconfigured")
	}
	if !s.configured(cf, Conn{Token: "x"}, false) {
		t.Error("cloudflare-api with token must be configured")
	}
	// File-based (cloudflared) is configured once host file access is available.
	cfd := &Cloudflared{}
	readFile := func(context.Context, string) (io.ReadCloser, error) { return nil, nil }
	if s.configured(cfd, Conn{}, false) {
		t.Error("cloudflared without file access must be unconfigured")
	}
	if !s.configured(cfd, Conn{ReadFile: readFile}, false) {
		t.Error("cloudflared with file access must be configured")
	}
	// HTTP-admin adapters (traefik) are configured by an endpoint.
	tr := &Traefik{}
	if !s.configured(tr, Conn{}, true) {
		t.Error("traefik with endpoint must be configured")
	}
}

func TestSuggestedTargets(t *testing.T) {
	store := &cpStore{
		nodes: []db.Node{{ID: "n1", Host: "192.168.20.9"}},
		ports: []db.PortExposureRow{
			{ContainerID: "c1", NodeID: "n1", HostIP: "0.0.0.0", HostPort: 8096},
			{ContainerID: "c1", NodeID: "n1", HostIP: "127.0.0.1", HostPort: 9000},
			{ContainerID: "c1", NodeID: "n1", HostIP: "10.0.0.5", HostPort: 3000},
			{ContainerID: "other", NodeID: "n1", HostPort: 1},   // wrong container
			{ContainerID: "c1", NodeID: "n9", HostPort: 2},      // wrong node
		},
	}
	s := &Service{store: store}
	got := s.suggestedTargets(context.Background(), db.Container{ID: "c1", NodeID: "n1"}, store.nodes)
	want := []string{
		"http://10.0.0.5:3000",     // specific bind IP preferred, port 3000 sorts first
		"http://192.168.20.9:8096", // 0.0.0.0 → owner host
		// 127.0.0.1:9000 is loopback-only → not reachable by a remote proxy → skipped
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("suggestedTargets = %v, want %v", got, want)
	}
}

// cfNode wires a stub store with a configured cloudflare-api node pointed at the
// given fake Cloudflare server.
func cfNodeStore(t *testing.T, endpoint string) (*cpStore, *crypto.Cipher) {
	cipher := testCipher(t)
	cfg, _ := json.Marshal(map[string]string{"kind": "cloudflare-api", "account_id": "acc-123", "tunnel_id": "tun-1"})
	store := &cpStore{
		containers: map[string]db.Container{
			"graf": {ID: "graf", NodeID: "n-backend", Name: "grafana"},
		},
		byNode: map[string][]db.Container{
			"n-tunnel":  {}, // tunnel node runs no detectable proxy container
			"n-backend": {{ID: "graf", NodeID: "n-backend", Name: "grafana"}},
		},
		nodes: []db.Node{
			{ID: "n-tunnel", Host: "192.0.2.1"},
			{ID: "n-backend", Host: "198.51.100.20"},
		},
		ports: []db.PortExposureRow{
			{ContainerID: "graf", NodeID: "n-backend", HostIP: "198.51.100.20", HostPort: 3000, ContainerPort: 3000},
		},
		proxyCfg: map[string]db.ProxyConfig{
			"n-tunnel": {NodeID: "n-tunnel", Endpoint: endpoint, TokenEncrypted: sealToken(t, cipher, "test-token"), ConfigJSON: string(cfg)},
		},
	}
	return store, cipher
}

func TestContainerProxyRoutesAndAddTargets(t *testing.T) {
	srv := cfServer(t, string(dashboardConfigFixture(t)))
	store, cipher := cfNodeStore(t, srv.URL)
	s := New(store, cipher)

	st, err := s.ContainerProxy(context.Background(), "graf")
	if err != nil {
		t.Fatalf("ContainerProxy: %v", err)
	}
	// grafana.example.com → http://198.51.100.20:3000 resolves to container "graf".
	if len(st.Routes) != 1 || st.Routes[0].SourceHost != "grafana.example.com" {
		t.Fatalf("routes = %+v, want one grafana.example.com route", st.Routes)
	}
	// The cloudflare-api tunnel node is a create-capable add target.
	if len(st.AddTargets) != 1 || st.AddTargets[0].NodeID != "n-tunnel" || st.AddTargets[0].CFTunnelID != "tun-1" {
		t.Fatalf("add_targets = %+v, want the cloudflare-api tunnel node", st.AddTargets)
	}
	if len(st.SuggestedTargets) != 1 || st.SuggestedTargets[0] != "http://198.51.100.20:3000" {
		t.Errorf("suggested = %v", st.SuggestedTargets)
	}
}

func TestAddContainerProxyDryRun(t *testing.T) {
	srv := cfServer(t, string(dashboardConfigFixture(t))) // GET only; no PUT/POST expected
	store, cipher := cfNodeStore(t, srv.URL)
	s := New(store, cipher)

	plan, err := s.AddContainerProxy(context.Background(), AddRouteRequest{
		ProxyNodeID: "n-tunnel",
		SourceHost:  "grafana.kaylas.systems",
		TargetURL:   "http://198.51.100.20:3000",
		CreateDNS:   true,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("AddContainerProxy dry-run: %v", err)
	}
	if plan.Applied {
		t.Error("dry-run must not apply")
	}
	if !strings.Contains(plan.DNSRecord, "tun-1.cfargotunnel.com") || !strings.Contains(plan.DNSRecord, "grafana.kaylas.systems") {
		t.Errorf("dns preview = %q", plan.DNSRecord)
	}
	if plan.Adapter != "cloudflare-api" {
		t.Errorf("adapter = %q", plan.Adapter)
	}
}

func TestAddContainerProxyRejectsBadInputInDryRun(t *testing.T) {
	srv := cfServer(t, string(dashboardConfigFixture(t)))
	store, cipher := cfNodeStore(t, srv.URL)
	s := New(store, cipher)

	_, err := s.AddContainerProxy(context.Background(), AddRouteRequest{
		ProxyNodeID: "n-tunnel",
		SourceHost:  "not a hostname",
		TargetURL:   "http://10.0.0.1:80",
		DryRun:      true,
	})
	if !errors.Is(err, ErrInvalidHostname) {
		t.Fatalf("err = %v, want ErrInvalidHostname even on dry-run", err)
	}

	_, err = s.AddContainerProxy(context.Background(), AddRouteRequest{
		ProxyNodeID: "n-tunnel",
		SourceHost:  "ok.example.com",
		TargetURL:   "ssh://10.0.0.1:22",
		DryRun:      true,
	})
	if !errors.Is(err, ErrInvalidService) {
		t.Fatalf("err = %v, want ErrInvalidService", err)
	}
}

func TestAddContainerProxyGatingNoProxy(t *testing.T) {
	store := &cpStore{
		byNode: map[string][]db.Container{"n1": {}},
		nodes:  []db.Node{{ID: "n1"}},
	}
	s := New(store, testCipher(t))
	_, err := s.AddContainerProxy(context.Background(), AddRouteRequest{
		ProxyNodeID: "n1", SourceHost: "x.example.com", TargetURL: "http://10.0.0.1:80",
	})
	if err == nil {
		t.Fatal("want error adding a route to a node with no create-capable proxy")
	}
}

// TestAddContainerProxyApply exercises the real write path end-to-end through the
// service, asserting it reaches the fake Cloudflare API (PUT + DNS POST).
func TestAddContainerProxyApply(t *testing.T) {
	ws := newCFWriteServer(t, string(dashboardConfigFixture(t)))
	store, cipher := cfNodeStore(t, ws.srv.URL)
	s := New(store, cipher)

	plan, err := s.AddContainerProxy(context.Background(), AddRouteRequest{
		ProxyNodeID: "n-tunnel",
		SourceHost:  "newapp.example.com",
		TargetURL:   "http://198.51.100.20:3000",
		CreateDNS:   true,
	})
	if err != nil {
		t.Fatalf("AddContainerProxy apply: %v", err)
	}
	if !plan.Applied || plan.Rule == nil {
		t.Fatalf("plan = %+v, want applied with a rule", plan)
	}
	if ws.putBody == nil {
		t.Error("ingress PUT did not reach the API")
	}
	if ws.dnsPosted == nil {
		t.Error("DNS record was not created")
	}
}
