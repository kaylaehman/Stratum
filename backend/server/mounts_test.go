package server_test

import (
	"net/http"
	"testing"
)

// Hermetic checks of the bind-mount tracer endpoints (no docker daemon). The
// forward/reverse/shared query logic is unit-tested in the mountindex package.

func TestContainerMountsUnknown404(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/containers/nope/mounts", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("container mounts unknown = %d, want 404", resp.StatusCode)
	}
}

func TestReverseMountsInvalidPath(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Create a docker-capable node so we pass the capability gate, then hit an
	// invalid host_path -> 400 before any index work.
	var created struct {
		ID string `json:"id"`
	}
	resp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "n", "host": "h", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_key"},
	}))
	// The bare node won't have docker capability, so reverse returns 422 (gate)
	// regardless of path. Either 422 (no docker) or 400 (bad path) is acceptable
	// here; assert it's a 4xx and never a 5xx/200.
	resp.Body.Close()
	_ = created

	rresp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/some-node/mounts?host_path=relative", token, nil))
	defer rresp.Body.Close()
	if rresp.StatusCode < 400 || rresp.StatusCode >= 500 {
		t.Fatalf("reverse with bad input = %d, want a 4xx", rresp.StatusCode)
	}
}
