package server_test

import (
	"net/http"
	"testing"
)

func TestSetCronValidation(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Invalid usernames are rejected before any SSH attempt.
	for _, body := range []map[string]string{
		{"user": "", "content": "0 3 * * * x"},
		{"user": "root; rm -rf /", "content": "x"},
		{"content": "x"}, // missing user
	} {
		resp, err := c.Do(authReq(t, http.MethodPut, srv.URL+"/api/nodes/n1/cron", token, body))
		if err != nil { t.Fatalf("request: %v", err) }
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("set cron %v = %d, want 400", body, resp.StatusCode)
		}
	}
}
