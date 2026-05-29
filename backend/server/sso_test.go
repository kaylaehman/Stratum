package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestSSOConfigCRUD covers upsert/list/delete + admin gating + secret-non-leak
// + validation for the SSO passthrough config layer (Feature F2).
func TestSSOConfigCRUD(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	// Real node for the FK.
	var node struct {
		ID string `json:"id"`
	}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", adminTok, map[string]any{
		"name": "ssonode", "host": "10.0.0.6", "ssh_port": 22, "credentials": map[string]string{"method": "ssh_key"},
	}))
	if err != nil { t.Fatalf("request: %v", err) }
	json.NewDecoder(resp.Body).Decode(&node)
	resp.Body.Close()

	// Admin-only.
	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/sso", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer list sso = %d, want 403", s)
	}

	// Invalid method rejected.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/sso", adminTok, map[string]any{
		"node_id": node.ID, "container_name": "grafana", "method": "magic",
	})); s != http.StatusBadRequest {
		t.Errorf("bad method = %d, want 400", s)
	}

	// Upsert an OIDC config with a client secret.
	var cfg struct {
		ID              string `json:"id"`
		HasClientSecret bool   `json:"has_client_secret"`
	}
	resp2, err := c.Do(authReq(t, http.MethodPut, srv.URL+"/api/sso", adminTok, map[string]any{
		"node_id": node.ID, "container_name": "grafana", "enabled": true, "method": "oidc",
		"provider_url": "https://auth.lan", "client_id": "stratum", "client_secret": "super-secret",
		"allowed_groups": []string{"homelab"}, "session_duration_secs": 3600,
	}))
	if err != nil { t.Fatalf("request: %v", err) }
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("upsert = %d, want 200", resp2.StatusCode)
	}
	var raw map[string]any
	json.NewDecoder(resp2.Body).Decode(&raw)
	resp2.Body.Close()
	if _, leaked := raw["client_secret"]; leaked {
		t.Error("response leaked client_secret")
	}
	if raw["has_client_secret"] != true {
		t.Errorf("has_client_secret = %v, want true", raw["has_client_secret"])
	}
	cfg.ID, _ = raw["id"].(string)
	if cfg.ID == "" {
		t.Fatal("no id returned")
	}

	// Second upsert WITHOUT a client_secret keeps the stored one.
	resp3, err := c.Do(authReq(t, http.MethodPut, srv.URL+"/api/sso", adminTok, map[string]any{
		"node_id": node.ID, "container_name": "grafana", "enabled": false, "method": "oidc",
		"provider_url": "https://auth.lan", "client_id": "stratum",
	}))
	if err != nil { t.Fatalf("request: %v", err) }
	json.NewDecoder(resp3.Body).Decode(&raw)
	resp3.Body.Close()
	if raw["has_client_secret"] != true {
		t.Error("omitting client_secret should keep the stored one")
	}

	// List shows exactly one config.
	resp4, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/sso", adminTok, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	var list struct {
		Configs []map[string]any `json:"configs"`
	}
	json.NewDecoder(resp4.Body).Decode(&list)
	resp4.Body.Close()
	if len(list.Configs) != 1 {
		t.Errorf("config count = %d, want 1", len(list.Configs))
	}

	// Delete.
	if s := status(t, c, authReq(t, http.MethodDelete, srv.URL+"/api/sso/"+cfg.ID, adminTok, nil)); s != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", s)
	}
}
