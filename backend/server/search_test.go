package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type searchResponse struct {
	Nodes      []map[string]any `json:"nodes"`
	Containers []map[string]any `json:"containers"`
	VMs        []map[string]any `json:"vms"`
	Bookmarks  []map[string]any `json:"bookmarks"`
}

func TestSearchEmptyQuery(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// No q => all groups empty arrays (not null), 200.
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/search", token, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("empty search = %d, want 200", resp.StatusCode)
	}
	var body searchResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Nodes == nil || body.Containers == nil || body.VMs == nil || body.Bookmarks == nil {
		t.Errorf("all groups must be arrays, got %+v", body)
	}
}

func TestSearchFindsNode(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Create a node, then search for it by name.
	createResp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "search-target-host", "host": "10.9.9.9", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_key"},
	}))
	createResp.Body.Close()

	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/search?q=search-target", token, nil))

	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	var body searchResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Nodes) != 1 {
		t.Fatalf("expected 1 node hit, got %d", len(body.Nodes))
	}
	if body.Nodes[0]["name"] != "search-target-host" {
		t.Errorf("hit = %v", body.Nodes[0])
	}
}
