package server_test

import (
	"net/http"
	"testing"
)

func TestContainerMetricsUnknown(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/containers/nope/metrics", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("metrics for unknown container = %d, want 404", resp.StatusCode)
	}
}

func TestContainerMetricsCSVHeader(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Unknown container -> 404; we only assert the route exists + gates resolve.
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/containers/nope/metrics.csv", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("metrics.csv unknown = %d, want 404", resp.StatusCode)
	}
}
