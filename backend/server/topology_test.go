package server_test

import (
	"net/http"
	"testing"
)

func TestNodeTopologyUnknownNode(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/nope/topology", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("topology for unknown node = %d, want 404", resp.StatusCode)
	}
}
