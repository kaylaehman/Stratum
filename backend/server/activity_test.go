package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestActivityListEmpty(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/activity", token, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/activity status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Entries    []map[string]any `json:"entries"`
		NextCursor string           `json:"next_cursor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Entries == nil {
		t.Error("entries should not be nil (may be empty)")
	}
	// next_cursor field must be present (even if empty string).
	// The JSON key "next_cursor" is always emitted.
}

func TestActivityListAfterMutation(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Create a node (which emits a node.create activity entry).
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "test-host", "host": "10.0.0.99", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_key"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create node status = %d", resp.StatusCode)
	}

	// Now query the activity log.
	listResp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/activity", token, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/activity status = %d", listResp.StatusCode)
	}

	var body struct {
		Entries []struct {
			Action string `json:"action"`
		} `json:"entries"`
	}
	json.NewDecoder(listResp.Body).Decode(&body)

	found := false
	for _, e := range body.Entries {
		if e.Action == "node.create" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected node.create entry in activity log, got entries: %+v", body.Entries)
	}
}

func TestActivityActions(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/activity/actions", token, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/activity/actions status = %d", resp.StatusCode)
	}

	var body struct {
		Actions []map[string]any `json:"actions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Actions) == 0 {
		t.Error("actions should not be empty")
	}
}

func TestActivityExportCSV(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/activity/export.csv", token, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/activity/export.csv status = %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv prefix", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	firstLine := strings.Split(strings.TrimSpace(string(body)), "\n")[0]
	if !strings.HasPrefix(firstLine, "timestamp,") {
		t.Errorf("first line of CSV = %q, want header starting with 'timestamp,'", firstLine)
	}
}

func TestActivityListInvalidDate(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/activity?from=not-a-date", token, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad date status = %d, want 400", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "invalid_date" {
		t.Errorf("error = %q, want invalid_date", body["error"])
	}
}
