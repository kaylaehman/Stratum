package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestAIConfigAndAskGating covers the AI endpoints: config is admin-only, ask
// is available to any authenticated user but returns a clear 400 when no
// provider is configured (the test server has no AI env/config).
func TestAIConfigAndAskGating(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	createUser(t, c, srv.URL, adminTok, "op", "operator")
	viewerTok := loginAs(t, c, srv.URL, "viewer")
	opTok := loginAs(t, c, srv.URL, "op")

	// Config GET is admin-only.
	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/ai/config", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer GET /ai/config = %d, want 403", s)
	}
	var cfg struct {
		Configured bool `json:"configured"`
		HasAPIKey  bool `json:"has_api_key"`
	}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/ai/config", adminTok, nil))
	json.NewDecoder(resp.Body).Decode(&cfg)
	resp.Body.Close()
	if cfg.Configured || cfg.HasAPIKey {
		t.Errorf("fresh config should be unconfigured: %+v", cfg)
	}

	// Ask is operator+: a viewer is blocked at the role gate (data egress).
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/ai/ask", viewerTok,
		map[string]string{"prompt": "hi"})); s != http.StatusForbidden {
		t.Errorf("viewer ask = %d, want 403", s)
	}
	// An operator passes the gate; with no provider configured -> 400.
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/ai/ask", opTok,
		map[string]string{"prompt": "why is my container down?"})); s != http.StatusBadRequest {
		t.Errorf("operator ask unconfigured = %d, want 400", s)
	}

	// Admin configures an Ollama endpoint; config now reports configured.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/ai/config", adminTok, map[string]any{
		"provider": "ollama", "ollama_base_url": "http://localhost:11434", "ollama_model": "llama3",
	})); s != http.StatusOK {
		t.Errorf("admin set config = %d, want 200", s)
	}
	resp, _ = c.Do(authReq(t, http.MethodGet, srv.URL+"/api/ai/config", adminTok, nil))
	json.NewDecoder(resp.Body).Decode(&cfg)
	resp.Body.Close()
	if !cfg.Configured {
		t.Error("config should be configured after setting an ollama endpoint")
	}

	// A bad provider/url is rejected.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/ai/config", adminTok, map[string]any{
		"provider": "ollama", "ollama_base_url": "ftp://nope",
	})); s != http.StatusBadRequest {
		t.Errorf("bad ollama url = %d, want 400", s)
	}
}
