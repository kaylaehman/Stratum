package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestProxyEndpointAdminGate verifies the reverse-proxy endpoints are admin-only
// and that a node-less server reports "no proxy detected" with the supported
// catalog, and rejects a bad config endpoint.
func TestProxyEndpointAdminGate(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/nodes/x/proxy", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer GET proxy = %d, want 403", s)
	}

	// Admin: unknown node has no containers -> no proxy detected, catalog present.
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/x/proxy", adminTok, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	var st struct {
		Detected  string `json:"detected"`
		Supported []struct {
			Name string `json:"name"`
		} `json:"supported"`
	}
	json.NewDecoder(resp.Body).Decode(&st)
	resp.Body.Close()
	if st.Detected != "" {
		t.Errorf("detected = %q, want empty", st.Detected)
	}
	if len(st.Supported) < 4 {
		t.Errorf("supported tools = %d, want the full catalog", len(st.Supported))
	}

	// Create a real node so proxy-config upsert satisfies the FK.
	var node struct {
		ID string `json:"id"`
	}
	resp2, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", adminTok, map[string]any{
		"name": "bare", "host": "10.0.0.9", "ssh_port": 22, "credentials": map[string]string{"method": "ssh_key"},
	}))
	if err != nil { t.Fatalf("request: %v", err) }
	json.NewDecoder(resp2.Body).Decode(&node)
	resp2.Body.Close()
	cfgURL := srv.URL + "/api/nodes/" + node.ID + "/proxy/config"

	// Bad endpoint (has a path) is rejected before any store write.
	if s := status(t, c, authReq(t, http.MethodPut, cfgURL, adminTok,
		map[string]any{"endpoint": "http://h/smuggled"})); s != http.StatusBadRequest {
		t.Errorf("bad endpoint = %d, want 400", s)
	}
	// Valid host-only endpoint is accepted.
	if s := status(t, c, authReq(t, http.MethodPut, cfgURL, adminTok,
		map[string]any{"endpoint": "http://traefik.lan:8080"})); s != http.StatusOK {
		t.Errorf("good endpoint = %d, want 200", s)
	}
}
