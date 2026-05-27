package server_test

import (
	"net/http"
	"testing"
)

// TestResumableUploadEndpointsAdminGate verifies the chunked-upload endpoints
// require admin (fs writes are admin-only) and reject a bad offset.
func TestResumableUploadEndpointsAdminGate(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	base := srv.URL + "/api/nodes/missing/fs/upload"

	// Viewer is blocked on every chunked endpoint.
	cases := []struct{ method, url string }{
		{http.MethodGet, base + "/status?path=/tmp/x"},
		{http.MethodPut, base + "/chunk?path=/tmp/x&offset=0"},
		{http.MethodDelete, base + "/chunk?path=/tmp/x"},
		{http.MethodPost, base + "/finish?path=/tmp/x"},
	}
	for _, tc := range cases {
		if s := status(t, c, authReq(t, tc.method, tc.url, viewerTok, nil)); s != http.StatusForbidden {
			t.Errorf("viewer %s %s = %d, want 403", tc.method, tc.url, s)
		}
	}

	// Admin with a bad offset -> 400 (before any node lookup).
	if s := status(t, c, authReq(t, http.MethodPut, base+"/chunk?path=/tmp/x&offset=abc", adminTok, nil)); s != http.StatusBadRequest {
		t.Errorf("admin bad offset = %d, want 400", s)
	}
}
