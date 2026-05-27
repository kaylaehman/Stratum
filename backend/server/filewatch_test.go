package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestFileWatchCRUD covers watch add/list/delete + events list + admin gating
// (Feature 22). The SSH-poll scan itself needs a live host, so only its gating
// is asserted here.
func TestFileWatchCRUD(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	// Real node for the FK.
	var node struct {
		ID string `json:"id"`
	}
	resp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", adminTok, map[string]any{
		"name": "watchnode", "host": "10.0.0.8", "ssh_port": 22, "credentials": map[string]string{"method": "ssh_key"},
	}))
	json.NewDecoder(resp.Body).Decode(&node)
	resp.Body.Close()
	base := srv.URL + "/api/nodes/" + node.ID + "/watches"

	// Watches are admin-only.
	if s := status(t, c, authReq(t, http.MethodGet, base, viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer list watches = %d, want 403", s)
	}
	// Invalid (relative) path rejected.
	if s := status(t, c, authReq(t, http.MethodPost, base, adminTok, map[string]any{"path": "etc/passwd"})); s != http.StatusBadRequest {
		t.Errorf("relative path = %d, want 400", s)
	}
	// Add a watch.
	var wch struct {
		ID string `json:"id"`
	}
	resp2, _ := c.Do(authReq(t, http.MethodPost, base, adminTok, map[string]any{"path": "/etc", "recursive": true}))
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("add watch = %d, want 201", resp2.StatusCode)
	}
	json.NewDecoder(resp2.Body).Decode(&wch)
	resp2.Body.Close()

	// List shows it.
	resp3, _ := c.Do(authReq(t, http.MethodGet, base, adminTok, nil))
	var list struct {
		Watches []map[string]any `json:"watches"`
	}
	json.NewDecoder(resp3.Body).Decode(&list)
	resp3.Body.Close()
	if len(list.Watches) != 1 {
		t.Errorf("watch count = %d, want 1", len(list.Watches))
	}

	// Events list is admin-only and starts empty.
	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/fileevents", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer events = %d, want 403", s)
	}
	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/fileevents", adminTok, nil)); s != http.StatusOK {
		t.Errorf("admin events = %d, want 200", s)
	}

	// Scan is admin-gated (viewer blocked).
	if s := status(t, c, authReq(t, http.MethodPost, base+"/scan", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer scan = %d, want 403", s)
	}

	// Delete the watch.
	if s := status(t, c, authReq(t, http.MethodDelete, base+"/"+wch.ID, adminTok, nil)); s != http.StatusNoContent {
		t.Errorf("delete watch = %d, want 204", s)
	}
}
