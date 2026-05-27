package server_test

import (
	"net/http"
	"testing"
)

// Hermetic checks of the container/uid-analysis handler wiring (no live docker
// daemon). The DAC verdict and mismatch classification are unit-tested in the
// permissions package.

func TestInspectUnknownContainer404(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/containers/nope/", token, nil))
	defer resp.Body.Close()
	// chi trims trailing slash differently; hit the canonical path.
	resp2, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/containers/nope", token, nil))
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("inspect unknown container = %d, want 404", resp2.StatusCode)
	}
}

func TestUIDAnalysisUnknownContainer404(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/containers/nope/uid-analysis", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("uid-analysis unknown container = %d, want 404", resp.StatusCode)
	}
}

func TestFileUIDRequiresAuth(t *testing.T) {
	srv, _ := newNodeTestServer(t)
	c := &http.Client{}
	// No bearer token -> 401 (auth middleware) before the handler.
	resp, _ := c.Get(srv.URL + "/api/containers/x/file-uid?host_path=/etc/hosts")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("file-uid without auth = %d, want 401", resp.StatusCode)
	}
}
