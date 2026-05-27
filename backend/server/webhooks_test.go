package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestWebhookCRUD(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Empty list + available triggers present.
	listResp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/webhooks", token, nil))
	var list struct {
		Webhooks          []map[string]any `json:"webhooks"`
		AvailableTriggers []string         `json:"available_triggers"`
	}
	json.NewDecoder(listResp.Body).Decode(&list)
	listResp.Body.Close()
	if list.Webhooks == nil || len(list.AvailableTriggers) == 0 {
		t.Fatalf("expected webhooks array + triggers, got %+v", list)
	}

	// Invalid provider => 400.
	bad, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/webhooks", token, map[string]any{
		"name": "x", "url": "https://hooks.slack.com/services/x", "provider": "teams",
	}))
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid provider = %d, want 400", bad.StatusCode)
	}

	// SSRF guard: a non-provider host (or http) must be rejected.
	for _, badURL := range []string{"http://hooks.slack.com/x", "https://169.254.169.254/latest", "https://evil.example.com/hook"} {
		ssrf, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/webhooks", token, map[string]any{
			"name": "x", "url": badURL, "provider": "slack",
		}))
		ssrf.Body.Close()
		if ssrf.StatusCode != http.StatusBadRequest {
			t.Errorf("SSRF url %q = %d, want 400", badURL, ssrf.StatusCode)
		}
	}

	// Create (valid Slack webhook host).
	createResp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/webhooks", token, map[string]any{
		"name": "alerts", "url": "https://hooks.slack.com/services/T/B/xyz", "provider": "slack",
		"triggers": []string{"port.new"}, "enabled": true,
	}))
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d, want 201", createResp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	// Update.
	upd, _ := c.Do(authReq(t, http.MethodPut, srv.URL+"/api/webhooks/"+created.ID, token, map[string]any{
		"name": "alerts2", "url": "https://discord.com/api/webhooks/123/abc", "provider": "discord", "enabled": false,
	}))
	upd.Body.Close()
	if upd.StatusCode != http.StatusNoContent {
		t.Errorf("update = %d, want 204", upd.StatusCode)
	}

	// Delete.
	del, _ := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/webhooks/"+created.ID, token, nil))
	del.Body.Close()
	if del.StatusCode != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", del.StatusCode)
	}
	// Delete again => 404.
	del2, _ := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/webhooks/"+created.ID, token, nil))
	del2.Body.Close()
	if del2.StatusCode != http.StatusNotFound {
		t.Errorf("delete missing = %d, want 404", del2.StatusCode)
	}
}
