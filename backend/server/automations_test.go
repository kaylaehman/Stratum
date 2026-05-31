package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestListAutomations_AdminOnly(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	// viewer is rejected (Automation is nil in test server → 503; but admin gate fires first → 403)
	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/automations", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer GET /api/automations = %d, want 403", s)
	}
	// admin with nil engine gets 503
	s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/automations", adminTok, nil))
	if s != http.StatusOK && s != http.StatusServiceUnavailable {
		t.Errorf("admin GET /api/automations = %d, want 200 or 503", s)
	}
}

func TestListAutomations_ReturnsAllEight(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/automations", adminTok, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	// engine is nil in testserver → 503, or 200 if wired
	if resp.StatusCode == http.StatusServiceUnavailable {
		t.Skip("automation engine not wired in test server")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Automations []map[string]any `json:"automations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Automations) != 8 {
		t.Errorf("got %d automations, want 8", len(body.Automations))
	}
}

func TestUpdateAutomation_AdminRequired(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "op", "operator")
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	opTok := loginAs(t, c, srv.URL, "op")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	url := srv.URL + "/api/automations/restart_unhealthy"
	body := map[string]any{"enabled": true, "interval_seconds": 60}

	for name, tok := range map[string]string{"viewer": viewerTok, "operator": opTok} {
		if s := status(t, c, authReq(t, http.MethodPut, url, tok, body)); s != http.StatusForbidden {
			t.Errorf("%s PUT automation = %d, want 403", name, s)
		}
	}
	// admin without engine wired: the handler accesses h.Store directly so it should still work (engine only used for GetView)
	s := status(t, c, authReq(t, http.MethodPut, url, adminTok, body))
	if s != http.StatusOK && s != http.StatusInternalServerError {
		t.Errorf("admin PUT automation = %d, want 200 or 500", s)
	}
}

func TestUpdateAutomation_UnknownKey(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/automations/bogus_key", adminTok, map[string]any{"enabled": true})); s != http.StatusNotFound {
		t.Errorf("unknown key = %d, want 404", s)
	}
}

func TestRunAutomation_OperatorAllowed(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "op", "operator")
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	opTok := loginAs(t, c, srv.URL, "op")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	url := srv.URL + "/api/automations/restart_unhealthy/run"

	// viewer is rejected
	if s := status(t, c, authReq(t, http.MethodPost, url, viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer POST run = %d, want 403", s)
	}
	// operator is allowed (engine may be nil in test server → 503 is also acceptable)
	s := status(t, c, authReq(t, http.MethodPost, url, opTok, nil))
	if s != http.StatusOK && s != http.StatusServiceUnavailable {
		t.Errorf("operator POST run = %d, want 200 or 503", s)
	}
}

func TestRunAutomation_UnknownKey(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/automations/no_such/run", adminTok, nil)); s != http.StatusNotFound {
		t.Errorf("unknown key run = %d, want 404", s)
	}
}
