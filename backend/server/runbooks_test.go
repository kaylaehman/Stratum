package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestRunbooksCRUD covers create/list/update/delete + role gating (Feature F9).
func TestRunbooksCRUD(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	// Viewer can read but not create.
	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/runbooks", viewerTok, nil)); s != http.StatusOK {
		t.Errorf("viewer list = %d, want 200", s)
	}
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/runbooks", viewerTok,
		map[string]any{"name": "x"})); s != http.StatusForbidden {
		t.Errorf("viewer create = %d, want 403", s)
	}

	// Admin creates a runbook.
	var rb struct {
		ID               string   `json:"id"`
		Steps            []string `json:"steps"`
		RequiresApproval bool     `json:"requires_approval"`
	}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/runbooks", adminTok, map[string]any{
		"name":               "Jellyfin permission reset",
		"description":        "Fix /config bind-mount UID mismatch",
		"trigger_conditions": []string{"jellyfin permission error in logs"},
		"steps":              []string{"check bind mount UIDs", "compare container UID", "chown if mismatch"},
		"requires_approval":  true,
	}))
	if err != nil { t.Fatalf("request: %v", err) }
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d, want 201", resp.StatusCode)
	}
	json.NewDecoder(resp.Body).Decode(&rb)
	resp.Body.Close()
	if len(rb.Steps) != 3 || !rb.RequiresApproval {
		t.Errorf("created runbook = %+v", rb)
	}

	// Name is required.
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/runbooks", adminTok, map[string]any{"description": "no name"})); s != http.StatusBadRequest {
		t.Errorf("no-name = %d, want 400", s)
	}

	// Update.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/runbooks/"+rb.ID, adminTok,
		map[string]any{"name": "Jellyfin perm reset v2", "steps": []string{"a", "b"}})); s != http.StatusOK {
		t.Errorf("update = %d, want 200", s)
	}

	// Delete.
	if s := status(t, c, authReq(t, http.MethodDelete, srv.URL+"/api/runbooks/"+rb.ID, adminTok, nil)); s != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", s)
	}
}
