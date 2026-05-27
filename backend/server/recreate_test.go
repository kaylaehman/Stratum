package server_test

import (
	"net/http"
	"testing"
)

// TestRecreateRoutesAdminGateAnd404 verifies the update/snapshot/rollback
// endpoints are admin-gated and return 404 for an unknown container (the gate
// runs before resolution, so a non-admin is 403 even on a missing container).
func TestRecreateRoutesAdminGateAnd404(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	missing := srv.URL + "/api/containers/does-not-exist"

	// Admin passes the gate, then 404 (no such container).
	for _, path := range []string{"/update", "/snapshot", "/rollback/snap-x"} {
		if s := status(t, c, authReq(t, http.MethodPost, missing+path, adminTok, nil)); s != http.StatusNotFound {
			t.Errorf("admin POST %s = %d, want 404", path, s)
		}
	}

	// Viewer is blocked at the admin gate (403) before resolution.
	for _, path := range []string{"/update", "/snapshot", "/rollback/snap-x"} {
		if s := status(t, c, authReq(t, http.MethodPost, missing+path, viewerTok, nil)); s != http.StatusForbidden {
			t.Errorf("viewer POST %s = %d, want 403", path, s)
		}
	}

	// Snapshots list is a read: any role, 404 for unknown container.
	if s := status(t, c, authReq(t, http.MethodGet, missing+"/snapshots", viewerTok, nil)); s != http.StatusNotFound {
		t.Errorf("viewer GET /snapshots = %d, want 404", s)
	}
}
