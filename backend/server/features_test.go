package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestFeatureFlags covers the catalog list, admin-only toggle, default
// resolution, and unknown-key rejection.
func TestFeatureFlags(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	// Any authed user can read the catalog.
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/features", viewerTok, nil))
	var body struct {
		Features []struct {
			Key     string `json:"key"`
			Enabled bool   `json:"enabled"`
			Default bool   `json:"default"`
		} `json:"features"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if len(body.Features) < 8 {
		t.Fatalf("catalog has %d features, want the full set", len(body.Features))
	}
	// sso_passthrough defaults off; reverse_proxy defaults on.
	defaults := map[string]bool{}
	for _, f := range body.Features {
		defaults[f.Key] = f.Enabled
	}
	if defaults["feature.sso_passthrough"] {
		t.Error("sso_passthrough should default disabled")
	}
	if !defaults["feature.reverse_proxy"] {
		t.Error("reverse_proxy should default enabled")
	}

	// Viewer can't toggle.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/features/feature.chat_integration", viewerTok,
		map[string]bool{"enabled": true})); s != http.StatusForbidden {
		t.Errorf("viewer toggle = %d, want 403", s)
	}
	// Unknown key rejected.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/features/feature.bogus", adminTok,
		map[string]bool{"enabled": true})); s != http.StatusBadRequest {
		t.Errorf("unknown key = %d, want 400", s)
	}
	// Admin enables chat_integration (was default off) and it sticks.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/features/feature.chat_integration", adminTok,
		map[string]bool{"enabled": true})); s != http.StatusOK {
		t.Errorf("admin toggle = %d, want 200", s)
	}
	resp, _ = c.Do(authReq(t, http.MethodGet, srv.URL+"/api/features", adminTok, nil))
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	for _, f := range body.Features {
		if f.Key == "feature.chat_integration" && !f.Enabled {
			t.Error("chat_integration should be enabled after toggle")
		}
	}
}
