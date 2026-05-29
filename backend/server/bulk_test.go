package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestBulkInvalidRequest(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	cases := []map[string]any{
		{"action": "pull", "container_ids": []string{"x"}}, // unsupported action
		{"action": "start", "container_ids": []string{}},   // empty id list
		{"container_ids": []string{"x"}},                    // missing action
	}
	for _, body := range cases {
		resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/containers/bulk", token, body))
		if err != nil { t.Fatalf("request: %v", err) }
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("bulk %v = %d, want 400", body, resp.StatusCode)
		}
	}
}

func TestBulkDryRunNotFound(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/containers/bulk", token, map[string]any{
		"action": "stop", "container_ids": []string{"nope1", "nope2"}, "dry_run": true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dry-run = %d, want 200", resp.StatusCode)
	}
	var body struct {
		DryRun  bool `json:"dry_run"`
		Results []struct {
			ContainerID string `json:"container_id"`
			Result      string `json:"result"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.DryRun || len(body.Results) != 2 {
		t.Fatalf("expected dry_run + 2 results, got %+v", body)
	}
	for _, r := range body.Results {
		if r.Result != "not_found" {
			t.Errorf("unknown container result = %q, want not_found", r.Result)
		}
	}
}
