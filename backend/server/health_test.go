package server_test

import (
	"net/http"
	"testing"
)

func TestContainerHealthUnknown(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/containers/nope/health", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("health for unknown container = %d, want 404", resp.StatusCode)
	}
}
