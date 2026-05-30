package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestIncidentTimelineUnauth ensures the endpoint rejects unauthenticated callers.
func TestIncidentTimelineUnauth(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/incidents/timeline")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		t.Errorf("unauthenticated request = %d, want 401 or 403", resp.StatusCode)
	}
}

// TestIncidentTimelineAuth verifies an authenticated user gets a valid response.
func TestIncidentTimelineAuth(t *testing.T) {
	srv, token := newNodeTestServer(t)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodGet, srv.URL+"/api/incidents/timeline", token, nil))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}

	var payload struct {
		Entries []any  `json:"entries"`
		From    string `json:"from"`
		To      string `json:"to"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.From == "" {
		t.Error("response missing 'from' field")
	}
	if payload.To == "" {
		t.Error("response missing 'to' field")
	}
	// On an empty store there should be zero entries (not nil).
	if payload.Entries == nil {
		t.Error("'entries' should be an array, got null")
	}
}

// TestIncidentTimelineInvalidDate returns 400 for a bad 'from' parameter.
func TestIncidentTimelineInvalidDate(t *testing.T) {
	srv, token := newNodeTestServer(t)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodGet, srv.URL+"/api/incidents/timeline?from=not-a-date", token, nil))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad from param = %d, want 400", resp.StatusCode)
	}
}

// TestIncidentTimelineNodeFilter verifies node_id query param is accepted.
func TestIncidentTimelineNodeFilter(t *testing.T) {
	srv, token := newNodeTestServer(t)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodGet, srv.URL+"/api/incidents/timeline?node_id=nonexistent-node", token, nil))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	// An unknown node_id should still return 200 with an empty entries list.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
}
