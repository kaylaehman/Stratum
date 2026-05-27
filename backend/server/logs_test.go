package server_test

import (
	"net/http"
	"testing"
)

// Hermetic checks of the logs subscribe gates (no ws connection / docker daemon).
// The tailer + refcount + auth logic is unit-tested in the logtail package.

func TestLogsSubscribeRejectsForeignWSClient(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// A ws_client_id that the hub doesn't know (or isn't owned by this user) ->
	// the server must never grant it a log topic.
	resp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/logs/subscribe", token, map[string]string{
		"container_id": "whatever", "ws_client_id": "not-a-real-client",
	}))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("subscribe with foreign ws client = %d, want 403", resp.StatusCode)
	}
}

func TestLogsSubscribeRequiresFields(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/logs/subscribe", token, map[string]string{}))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("subscribe without fields = %d, want 400", resp.StatusCode)
	}
}
