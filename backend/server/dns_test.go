package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestDNSEndpointAdminGate verifies the DNS endpoints are admin-only, report
// "no tool detected" + catalog for a node-less server, and validate config.
func TestDNSEndpointAdminGate(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/nodes/x/dns", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer GET dns = %d, want 403", s)
	}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/x/dns", adminTok, nil))
	var st struct {
		Detected  string `json:"detected"`
		Supported []struct {
			Name string `json:"name"`
		} `json:"supported"`
	}
	json.NewDecoder(resp.Body).Decode(&st)
	resp.Body.Close()
	if st.Detected != "" || len(st.Supported) < 3 {
		t.Errorf("status = %+v, want empty detection + catalog", st)
	}

	// Real node for the config FK.
	var node struct {
		ID string `json:"id"`
	}
	resp2, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", adminTok, map[string]any{
		"name": "dnsnode", "host": "10.0.0.7", "ssh_port": 22, "credentials": map[string]string{"method": "ssh_key"},
	}))
	json.NewDecoder(resp2.Body).Decode(&node)
	resp2.Body.Close()
	cfg := srv.URL + "/api/nodes/" + node.ID + "/dns/config"

	if s := status(t, c, authReq(t, http.MethodPut, cfg, adminTok, map[string]any{"endpoint": "http://h/x?y="})); s != http.StatusBadRequest {
		t.Errorf("bad endpoint = %d, want 400", s)
	}
	if s := status(t, c, authReq(t, http.MethodPut, cfg, adminTok, map[string]any{"endpoint": "http://adguard.lan", "token": "secret"})); s != http.StatusOK {
		t.Errorf("good endpoint = %d, want 200", s)
	}
}
