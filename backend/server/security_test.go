package server_test

import (
	"net/http"
	"testing"
)

// Hermetic checks of the security endpoints' gates (no docker daemon). The scan
// classification + flag enumeration are unit-tested in the security package.

func TestSecurityBadgesEmpty(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/containers/security-badges", token, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("security-badges = %d, want 200", resp.StatusCode)
	}
}

func TestPrivilegedListOK(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// No flagged containers yet -> 200 with an empty list (admin gate passes).
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/security/privileged", token, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("privileged = %d, want 200", resp.StatusCode)
	}
}

func TestAcknowledgeRequiresContainer(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/security/acknowledge", token, map[string]string{
		"container_id": "nope", "flag_type": "privileged",
	}))
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("acknowledge unknown container = %d, want 404", resp.StatusCode)
	}
}
