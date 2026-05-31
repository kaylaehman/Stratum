package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestVolumesListEmpty(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// No docker-capable nodes registered => empty list, 200 (not an error).
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/volumes", token, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/volumes = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Volumes []map[string]any `json:"volumes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Volumes == nil {
		t.Error("volumes should be an empty array, not null")
	}
}

func TestRemoveVolumeMissingNode(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Removing on an unknown node: the docker provider errors => 500 remove_failed
	// (not a panic). This exercises the audited route + admin gate end to end.
	resp, err := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/nodes/nope/volumes/data", token, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("DELETE volume on missing node = %d, want 500", resp.StatusCode)
	}
}

// TestPruneUnusedVolumesEmpty: with no docker-capable nodes registered, an
// all-nodes prune (empty body) succeeds with empty results and zero counts.
// Exercises the audited route + admin gate + step-up end to end.
func TestPruneUnusedVolumesEmpty(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/volumes/prune-unused", token, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/volumes/prune-unused = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Results      []map[string]any `json:"results"`
		RemovedCount int              `json:"removed_count"`
		FailedCount  int              `json:"failed_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Results == nil {
		t.Error("results should be an empty array, not null")
	}
	if body.RemovedCount != 0 || body.FailedCount != 0 {
		t.Errorf("counts = %d removed / %d failed, want 0/0", body.RemovedCount, body.FailedCount)
	}
}

// TestPruneUnusedVolumesRBAC: the prune is admin-only. A viewer and an operator
// are both rejected at the admin gate (403); the admin passes the gate.
func TestPruneUnusedVolumesRBAC(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "op", "operator")
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	opTok := loginAs(t, c, srv.URL, "op")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	url := srv.URL + "/api/volumes/prune-unused"
	for name, tok := range map[string]string{"viewer": viewerTok, "operator": opTok} {
		if s := status(t, c, authReq(t, http.MethodPost, url, tok, map[string]string{})); s != http.StatusForbidden {
			t.Errorf("%s prune = %d, want 403 (admin-only)", name, s)
		}
	}
	if s := status(t, c, authReq(t, http.MethodPost, url, adminTok, map[string]string{})); s != http.StatusOK {
		t.Errorf("admin prune = %d, want 200", s)
	}
}
