package server_test

import (
	"net/http"
	"testing"
)

// These exercise the fs API handler gates that run before any SSH/SFTP I/O
// (admin gate, confirm token, path validation, required fields). Deep fs
// behavior (deny-list, atomic write, stale-412, upload cap) is unit-tested in
// the fs package against a fake provider.

func TestFSDeleteRequiresConfirm(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// No confirm=yes -> 400 before any filesystem op.
	resp, _ := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/nodes/n/fs?path=/home/x", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("delete without confirm = %d, want 400", resp.StatusCode)
	}
}

func TestFSListInvalidPath(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Relative path is rejected by ValidatePath before any dial.
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/n/fs?path=relative/path", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("list relative path = %d, want 400", resp.StatusCode)
	}
}

func TestFSMkdirRequiresPath(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes/n/fs/mkdir", token, map[string]string{}))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("mkdir without path = %d, want 400", resp.StatusCode)
	}
}
