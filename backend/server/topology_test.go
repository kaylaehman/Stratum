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
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("topology for unknown node = %d, want 404", resp.StatusCode)
	}
}

func TestNodeTopologyNoDocker(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Create a node with no docker capability (no creds → probe finds nothing).
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

	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("topology on non-docker node = %d, want 409", resp.StatusCode)
	}
}
