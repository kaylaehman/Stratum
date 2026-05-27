package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestScriptCRUDAndRunValidation(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Empty list.
	lr, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/scripts", token, nil))
	var list struct {
		Scripts []map[string]any `json:"scripts"`
	}
	json.NewDecoder(lr.Body).Decode(&list)
	lr.Body.Close()
	if list.Scripts == nil {
		t.Fatal("scripts should be an array")
	}

	// Missing content => 400.
	bad, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/scripts", token, map[string]string{"name": "x"}))
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("incomplete create = %d, want 400", bad.StatusCode)
	}

	// Create.
	cr, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/scripts", token, map[string]string{
		"name": "uptime", "content": "uptime",
	}))
	if cr.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d, want 201", cr.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(cr.Body).Decode(&created)
	cr.Body.Close()

	// Run with no node_ids => 400.
	rr, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/scripts/"+created.ID+"/run", token, map[string]any{"node_ids": []string{}}))
	rr.Body.Close()
	if rr.StatusCode != http.StatusBadRequest {
		t.Errorf("run without nodes = %d, want 400", rr.StatusCode)
	}

	// Run against an unreachable node => 200 with a per-node failure result.
	run, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/scripts/"+created.ID+"/run", token, map[string]any{"node_ids": []string{"nope"}}))
	defer run.Body.Close()
	if run.StatusCode != http.StatusOK {
		t.Fatalf("run = %d, want 200", run.StatusCode)
	}
	var res struct {
		Results []struct {
			NodeID string `json:"node_id"`
			OK     bool   `json:"ok"`
		} `json:"results"`
	}
	json.NewDecoder(run.Body).Decode(&res)
	if len(res.Results) != 1 || res.Results[0].OK {
		t.Errorf("expected 1 failed per-node result, got %+v", res.Results)
	}

	// Delete.
	dr, _ := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/scripts/"+created.ID, token, nil))
	dr.Body.Close()
	if dr.StatusCode != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", dr.StatusCode)
	}
}
