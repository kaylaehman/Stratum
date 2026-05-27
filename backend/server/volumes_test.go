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
