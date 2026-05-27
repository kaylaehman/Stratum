package server_test

import (
	"net/http"
	"testing"
)

// TestCertsEndpointAdminGate verifies the cert inventory + rescan are admin-only
// and return an (empty) list for a node-less test server.
func TestCertsEndpointAdminGate(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/certs", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer GET /certs = %d, want 403", s)
	}
	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/certs", adminTok, nil)); s != http.StatusOK {
		t.Errorf("admin GET /certs = %d, want 200", s)
	}
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/certs/rescan", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer rescan = %d, want 403", s)
	}
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/certs/rescan", adminTok, nil)); s != http.StatusOK {
		t.Errorf("admin rescan = %d, want 200", s)
	}
}
