package proxy

import (
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
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

// TestResolveTargetNameMatchScopedToNode verifies a service-name match never
// crosses node boundaries (v1 scopes resolution to the current node).
func TestResolveTargetNameMatchScopedToNode(t *testing.T) {
	containers := []db.Container{
		{ID: "inv-other", NodeID: "node-2", Name: "jellyfin"},
	}
	// Resolving on node-1 against a container that only exists on node-2.
	if got := resolveTarget("http://jellyfin:8096", "node-1", containers, nil); got != nil {
		t.Errorf("resolveTarget cross-node = %+v, want nil (current-node scope)", got)
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
