package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestNodeTopologyUnknownNode(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/nope/topology", token, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("topology for unknown node = %d, want 404", resp.StatusCode)
	}
}

// TestNodeTopologyNoDocker verifies that requesting topology for a node without
// Docker capability returns HTTP 200 with an empty topology and a non-empty
// docker_error field. Previously this returned 409 Conflict, but the fix
// ensures the backend never infers "unreachable" from a Docker dial failure:
// node reachability is communicated via node_status (poller-authoritative),
// not via the HTTP status code.
func TestNodeTopologyNoDocker(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Create a node with no docker capability (no docker endpoint configured).
	createResp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "ssh-only", "host": "10.0.0.50", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_key"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()
	if created.ID == "" {
		t.Fatal("node create returned no id")
	}

	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/"+created.ID+"/topology", token, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	// The response must be 200 — not 409. Capability gating no longer happens at
	// the HTTP layer; the service absorbs the Docker failure into docker_error.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("topology on non-docker node = %d, want 200", resp.StatusCode)
	}
	var body struct {
		NodeStatus  string `json:"node_status"`
		DockerError string `json:"docker_error"`
		Networks    []any  `json:"networks"`
		Containers  []any  `json:"containers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	// Docker is not configured on this node — docker_error must be non-empty.
	if body.DockerError == "" {
		t.Error("docker_error should be non-empty for a node with no Docker endpoint")
	}
	// Networks and containers must be empty (no Docker data available).
	if len(body.Networks) != 0 {
		t.Errorf("networks should be empty, got %d", len(body.Networks))
	}
	if len(body.Containers) != 0 {
		t.Errorf("containers should be empty, got %d", len(body.Containers))
	}
}
