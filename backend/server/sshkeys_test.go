package server_test

import (
	"net/http"
	"testing"
)

func TestSSHKeyDeleteValidation(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Missing fingerprint / bad path => 400 (before any SSH attempt).
	for _, body := range []map[string]string{
		{"path": "/root/.ssh/authorized_keys"}, // no fingerprint
		{"fingerprint": "SHA256:x", "path": "/etc/passwd"}, // bad path
		{"fingerprint": "SHA256:x"}, // no path
	} {
		resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes/n1/sshkeys/delete", token, body))
		if err != nil { t.Fatalf("request: %v", err) }
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("delete %v = %d, want 400", body, resp.StatusCode)
		}
	}
}
