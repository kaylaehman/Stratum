package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestUpdatesListEmpty(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// No docker nodes => empty list, 200 (EnsureAll is best-effort).
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/updates", token, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/updates = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Updates []map[string]any `json:"updates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Updates == nil {
		t.Error("updates should be an array, not null")
	}
}

func TestUpdatesRescanAdminGate(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Admin token => 204 (no docker nodes, but the gate + action run).
	resp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/updates/rescan", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("rescan (admin) = %d, want 204", resp.StatusCode)
	}
}
