package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestAgentMemoryCRUD covers create/list/update(confirm)/delete + role gating
// for the AI agent-memory store (Feature F9).
func TestAgentMemoryCRUD(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	// Viewer can read memory but not create it (operator+).
	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/memory?scope=global", viewerTok, nil)); s != http.StatusOK {
		t.Errorf("viewer list memory = %d, want 200", s)
	}
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/memory", viewerTok,
		map[string]string{"scope": "global", "key": "k", "value": "v"})); s != http.StatusForbidden {
		t.Errorf("viewer create memory = %d, want 403", s)
	}

	// Admin creates a global memory.
	var created struct {
		ID        string `json:"id"`
		Confirmed bool   `json:"confirmed"`
		Source    string `json:"source"`
	}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/memory", adminTok, map[string]string{
		"scope": "global", "key": "access", "value": "prefers Tailscale over port forwarding",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create memory = %d, want 201", resp.StatusCode)
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if !created.Confirmed || created.Source != "user" {
		t.Errorf("user memory should be confirmed+user-sourced: %+v", created)
	}

	// Duplicate (scope,scope_id,key) is rejected.
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/memory", adminTok,
		map[string]string{"scope": "global", "key": "access", "value": "dup"})); s != http.StatusConflict {
		t.Errorf("duplicate key = %d, want 409", s)
	}

	// Update the value.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/memory/"+created.ID, adminTok,
		map[string]any{"value": "updated"})); s != http.StatusOK {
		t.Errorf("update memory = %d, want 200", s)
	}

	// List reflects one global memory.
	resp, _ = c.Do(authReq(t, http.MethodGet, srv.URL+"/api/memory?scope=global", adminTok, nil))
	var list struct {
		Memories []map[string]any `json:"memories"`
	}
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list.Memories) != 1 {
		t.Errorf("global memory count = %d, want 1", len(list.Memories))
	}

	// Delete.
	if s := status(t, c, authReq(t, http.MethodDelete, srv.URL+"/api/memory/"+created.ID, adminTok, nil)); s != http.StatusNoContent {
		t.Errorf("delete memory = %d, want 204", s)
	}
}
