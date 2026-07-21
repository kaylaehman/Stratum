package proxy

import (
	"testing"

	"github.com/KAE-Labs/stratum/backend/db"
)

func TestParseTargetURL(t *testing.T) {
	cases := []struct {
		raw      string
		wantHost string
		wantPort int
	}{
		{"http://localhost:8096", "localhost", 8096},
		{"http://jellyfin:8096", "jellyfin", 8096},
		{"http://192.168.20.9:8096", "192.168.20.9", 8096},
		{"https://app.example.com", "app.example.com", 443}, // scheme default port
		{"http://plex", "plex", 80},                         // scheme default port
		{"jellyfin:8096", "jellyfin", 8096},                 // scheme-less host:port
		{"192.168.1.5:3000", "192.168.1.5", 3000},           // scheme-less ip:port
		{"plex", "plex", 0},                                 // bare host, no port
		{"http://[::1]:9000", "::1", 9000},                  // IPv6 literal
		{"tcp://db:5432", "db", 5432},                       // non-http scheme
		{"http://localhost:8096/path", "localhost", 8096},   // path is ignored
		{"http_status:404", "", 0},                          // cloudflared catch-all
		{"", "", 0},                                         // empty
	}
	for _, tc := range cases {
		got := parseTargetURL(tc.raw)
		if got.host != tc.wantHost || got.port != tc.wantPort {
			t.Errorf("parseTargetURL(%q) = {host:%q port:%d}, want {host:%q port:%d}",
				tc.raw, got.host, got.port, tc.wantHost, tc.wantPort)
		}
	}
}

func TestResolveTarget(t *testing.T) {
	const node = "node-1"
	const otherNode = "node-2"

	containers := []db.Container{
		{ID: "inv-jelly", NodeID: node, Name: "jellyfin"},
		{ID: "inv-grafana", NodeID: node, Name: "grafana"},
		{ID: "inv-other", NodeID: otherNode, Name: "jellyfin"}, // same name, different node
	}
	ports := []db.PortExposureRow{
		// jellyfin publishes 8096 on all interfaces (localhost reachable).
		{NodeID: node, ContainerID: "inv-jelly", HostIP: "0.0.0.0", HostPort: 8096, ContainerPort: 8096, Protocol: "tcp"},
		// grafana publishes 3000 bound to a specific LAN IP only.
		{NodeID: node, ContainerID: "inv-grafana", HostIP: "192.168.20.9", HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"},
		// a port on the other node — must never match for node-1.
		{NodeID: otherNode, ContainerID: "inv-other", HostIP: "0.0.0.0", HostPort: 8096, ContainerPort: 8096, Protocol: "tcp"},
	}

	cases := []struct {
		name     string
		target   string
		wantNil  bool
		wantCID  string
		wantKind string
	}{
		{
			name:     "localhost+port -> published-port match",
			target:   "http://localhost:8096",
			wantCID:  "inv-jelly",
			wantKind: MatchLocalhostPort,
		},
		{
			name:     "127.0.0.1+port -> published-port match",
			target:   "http://127.0.0.1:8096",
			wantCID:  "inv-jelly",
			wantKind: MatchLocalhostPort,
		},
		{
			name:     "service name -> container_name match (this node)",
			target:   "http://jellyfin:8096",
			wantCID:  "inv-jelly",
			wantKind: MatchContainerName,
		},
		{
			name:     "host IP + port -> host_ip_port match",
			target:   "http://192.168.20.9:3000",
			wantCID:  "inv-grafana",
			wantKind: MatchHostIPPort,
		},
		{
			name:    "unknown service name -> unresolved",
			target:  "http://nonexistent:8096",
			wantNil: true,
		},
		{
			name:    "localhost + unpublished port -> unresolved",
			target:  "http://localhost:9999",
			wantNil: true,
		},
		{
			name:    "host IP that no container binds -> unresolved",
			target:  "http://10.0.0.5:3000",
			wantNil: true,
		},
		{
			name:    "catch-all status -> unresolved",
			target:  "http_status:404",
			wantNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveTarget(tc.target, node, containers, ports)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("resolveTarget(%q) = %+v, want nil", tc.target, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("resolveTarget(%q) = nil, want match %s", tc.target, tc.wantCID)
			}
			if got.NodeID != node {
				t.Errorf("NodeID = %q, want %q", got.NodeID, node)
			}
			if got.ContainerID != tc.wantCID {
				t.Errorf("ContainerID = %q, want %q", got.ContainerID, tc.wantCID)
			}
			if got.MatchKind != tc.wantKind {
				t.Errorf("MatchKind = %q, want %q", got.MatchKind, tc.wantKind)
			}
		})
	}
}

// TestResolveTargetNameCrossNode verifies a service-name match falls through to
// other nodes when no container with that name exists on the tunnel's node.
func TestResolveTargetNameCrossNode(t *testing.T) {
	containers := []db.Container{
		{ID: "inv-other", NodeID: "node-2", Name: "jellyfin"},
	}
	// Resolving on node-1; the only jellyfin container lives on node-2.
	got := resolveTarget("http://jellyfin:8096", "node-1", containers, nil)
	if got == nil {
		t.Fatal("resolveTarget cross-node name = nil, want node-2 jellyfin match")
	}
	if got.NodeID != "node-2" || got.ContainerID != "inv-other" || got.MatchKind != MatchContainerName {
		t.Errorf("got %+v, want node-2/inv-other/container_name", got)
	}
}

// TestResolveTargetNameMatchWithoutPorts verifies name resolution still works
// when no published-port data is available.
func TestResolveTargetNameMatchWithoutPorts(t *testing.T) {
	containers := []db.Container{
		{ID: "inv-jelly", NodeID: "node-1", Name: "jellyfin"},
	}
	got := resolveTarget("http://jellyfin:8096", "node-1", containers, nil)
	if got == nil || got.ContainerID != "inv-jelly" || got.MatchKind != MatchContainerName {
		t.Errorf("resolveTarget = %+v, want jellyfin container_name match", got)
	}
}

// TestResolveTargetComposeServiceName is the regression test for tunnels routing
// to Compose containers by their SERVICE alias: the ingress host "jellyfin" must
// match a container whose full Name is "media-jellyfin-1" via its compose service
// name — otherwise every Compose container behind a tunnel resolves to nothing
// and the proxy rule only shows at host level.
func TestResolveTargetComposeServiceName(t *testing.T) {
	containers := []db.Container{
		{ID: "inv-jelly", NodeID: "node-1", Name: "media-jellyfin-1", ComposeProject: "media", ComposeService: "jellyfin"},
	}
	got := resolveTarget("http://jellyfin:8096", "node-1", containers, nil)
	if got == nil || got.ContainerID != "inv-jelly" || got.MatchKind != MatchContainerName {
		t.Errorf("resolveTarget = %+v, want compose-service match on 'jellyfin'", got)
	}
}

// TestResolveTargetCrossNodeHostIP is the primary regression test for the bug:
// a cloudflared tunnel on node-A with a rule pointing at http://192.168.20.9:5006
// should resolve to the container on node-B whose host publishes port 5006 and
// whose HostIP binding is 192.168.20.9.
func TestResolveTargetCrossNodeHostIP(t *testing.T) {
	const tunnelNode = "node-a" // cloudflared runs here
	const targetNode = "node-b" // actual-budget runs here

	containers := []db.Container{
		{ID: "inv-tunnel-cf", NodeID: tunnelNode, Name: "cloudflared"},
		{ID: "inv-budget", NodeID: targetNode, Name: "actual-budget"},
	}
	ports := []db.PortExposureRow{
		// actual-budget on node-b publishes 5006 bound to the node's LAN IP.
		{NodeID: targetNode, ContainerID: "inv-budget", HostIP: "192.168.20.9", HostPort: 5006, ContainerPort: 5006, Protocol: "tcp"},
	}

	got := resolveTarget("http://192.168.20.9:5006", tunnelNode, containers, ports)
	if got == nil {
		t.Fatal("resolveTarget cross-node host-IP = nil, want actual-budget on node-b")
	}
	if got.NodeID != targetNode {
		t.Errorf("NodeID = %q, want %q", got.NodeID, targetNode)
	}
	if got.ContainerID != "inv-budget" {
		t.Errorf("ContainerID = %q, want inv-budget", got.ContainerID)
	}
	if got.MatchKind != MatchHostIPPort {
		t.Errorf("MatchKind = %q, want %q", got.MatchKind, MatchHostIPPort)
	}
}

// TestResolveTargetCrossNodeHostIPViaNodeHost tests the fallback path where
// the node registered with the IP as its Host field but the port binding uses
// 0.0.0.0 (not the specific IP). resolveTargetWithNodes must map the IP to the
// node via Node.Host and then match on host-port alone.
func TestResolveTargetCrossNodeHostIPViaNodeHost(t *testing.T) {
	const tunnelNode = "node-a"
	const targetNode = "node-b"

	containers := []db.Container{
		{ID: "inv-budget", NodeID: targetNode, Name: "actual-budget"},
	}
	// Port binding uses 0.0.0.0, not the specific IP — common on Docker hosts.
	ports := []db.PortExposureRow{
		{NodeID: targetNode, ContainerID: "inv-budget", HostIP: "0.0.0.0", HostPort: 5006, ContainerPort: 5006, Protocol: "tcp"},
	}
	nodes := []db.Node{
		{ID: tunnelNode, Host: "192.168.20.5"},
		{ID: targetNode, Host: "192.168.20.9"}, // node registered with its LAN IP
	}

	got := resolveTargetWithNodes("http://192.168.20.9:5006", tunnelNode, containers, ports, nodes)
	if got == nil {
		t.Fatal("resolveTargetWithNodes node-host fallback = nil, want actual-budget on node-b")
	}
	if got.NodeID != targetNode || got.ContainerID != "inv-budget" || got.MatchKind != MatchHostIPPort {
		t.Errorf("got %+v, want node-b/inv-budget/host_ip_port", got)
	}
}

// TestResolveTargetLocalhostStillTunnelNode ensures loopback targets are NOT
// redirected cross-node — they must still resolve on the tunnel's own node.
func TestResolveTargetLocalhostStillTunnelNode(t *testing.T) {
	const tunnelNode = "node-a"
	const otherNode = "node-b"

	containers := []db.Container{
		{ID: "inv-a", NodeID: tunnelNode, Name: "svc"},
		{ID: "inv-b", NodeID: otherNode, Name: "svc"},
	}
	ports := []db.PortExposureRow{
		{NodeID: tunnelNode, ContainerID: "inv-a", HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 8080, Protocol: "tcp"},
		{NodeID: otherNode, ContainerID: "inv-b", HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 8080, Protocol: "tcp"},
	}

	got := resolveTarget("http://localhost:8080", tunnelNode, containers, ports)
	if got == nil {
		t.Fatal("localhost resolve = nil, want inv-a on tunnel node")
	}
	if got.NodeID != tunnelNode || got.ContainerID != "inv-a" {
		t.Errorf("got %+v, want node-a/inv-a (loopback must not cross nodes)", got)
	}
}
