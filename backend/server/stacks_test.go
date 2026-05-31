package server_test

import (
	"net/http"
	"testing"
)

// TestStackLifecycleInvalidAction confirms the action allowlist is enforced at
// the HTTP boundary: a bad action is a 400 before the handler ever reaches the
// (test-unwired) Stacks service. Admin passes the operator gate and step-up.
func TestStackLifecycleInvalidAction(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	url := srv.URL + "/api/nodes/n1/stacks/proj/lifecycle"
	for _, action := range []string{"", "up", "down", "destroy"} {
		resp, err := c.Do(authReq(t, http.MethodPost, url, token, map[string]string{"action": action}))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("lifecycle action=%q = %d, want 400", action, resp.StatusCode)
		}
	}
}

// TestStackLifecycleMissingNode: a valid action on an unknown node is a 404
// (node lookup happens before the Stacks call, so this is safe with a nil Stacks
// in the test server). Exercises the audited route + operator gate end to end.
func TestStackLifecycleMissingNode(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	url := srv.URL + "/api/nodes/does-not-exist/stacks/proj/lifecycle"
	resp, err := c.Do(authReq(t, http.MethodPost, url, token, map[string]string{"action": "restart"}))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("lifecycle on missing node = %d, want 404", resp.StatusCode)
	}
}

// TestStackLifecycleRBAC: the operator gate matches container start/stop/restart.
// A viewer is rejected at the gate (403) before action validation; an operator
// passes the gate and only then hits 404 for the missing node.
func TestStackLifecycleRBAC(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "op", "operator")
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	opTok := loginAs(t, c, srv.URL, "op")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	url := srv.URL + "/api/nodes/does-not-exist/stacks/proj/lifecycle"
	body := map[string]string{"action": "restart"}

	if s := status(t, c, authReq(t, http.MethodPost, url, viewerTok, body)); s != http.StatusForbidden {
		t.Errorf("viewer lifecycle = %d, want 403", s)
	}
	if s := status(t, c, authReq(t, http.MethodPost, url, opTok, body)); s != http.StatusNotFound {
		t.Errorf("operator lifecycle = %d, want 404 (gate passed, node missing)", s)
	}
}
