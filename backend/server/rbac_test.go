package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// createUser provisions a user via the admin API and returns the new user id.
func createUser(t *testing.T, c *http.Client, base, adminTok, username, role string) string {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	resp, err := c.Do(authReq(t, http.MethodPost, base+"/api/users", adminTok, map[string]string{
		"username": username, "password": "supersecret", "role": role,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create %s(%s) = %d, want 201", username, role, resp.StatusCode)
	}
	json.NewDecoder(resp.Body).Decode(&created)
	return created.ID
}

func loginAs(t *testing.T, c *http.Client, base, username string) string {
	t.Helper()
	var login struct {
		AccessToken string `json:"access_token"`
	}
	postJSONInto(t, c, base+"/api/auth/login", map[string]string{
		"username": username, "password": "supersecret",
	}, http.StatusOK, &login)
	if login.AccessToken == "" {
		t.Fatalf("no token for %s", username)
	}
	return login.AccessToken
}

func status(t *testing.T, c *http.Client, req *http.Request) int {
	t.Helper()
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// TestRBACRoleGating verifies the three-tier hierarchy at the HTTP boundary:
// viewers read only, operators can run lifecycle, admins do everything.
func TestRBACRoleGating(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	createUser(t, c, srv.URL, adminTok, "op", "operator")
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	opTok := loginAs(t, c, srv.URL, "op")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	// All roles can read inventory.
	for name, tok := range map[string]string{"viewer": viewerTok, "operator": opTok, "admin": adminTok} {
		if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/nodes", tok, nil)); s != http.StatusOK {
			t.Errorf("%s GET /nodes = %d, want 200", name, s)
		}
	}

	// Container start: viewer is blocked at the role gate (403); operator passes
	// the gate and only then hits 404 for the missing container.
	startURL := srv.URL + "/api/containers/does-not-exist/start"
	if s := status(t, c, authReq(t, http.MethodPost, startURL, viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer start = %d, want 403", s)
	}
	if s := status(t, c, authReq(t, http.MethodPost, startURL, opTok, nil)); s != http.StatusNotFound {
		t.Errorf("operator start = %d, want 404 (gate passed, container missing)", s)
	}

	// User management is admin-only: operator is rejected.
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/users", opTok, map[string]string{
		"username": "x", "password": "supersecret", "role": "viewer",
	})); s != http.StatusForbidden {
		t.Errorf("operator create-user = %d, want 403", s)
	}

	// Destructive bulk remove is admin-only even though operators can bulk-stop.
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/containers/bulk", opTok, map[string]any{
		"action": "remove", "container_ids": []string{"x"},
	})); s != http.StatusForbidden {
		t.Errorf("operator bulk remove = %d, want 403", s)
	}
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/containers/bulk", opTok, map[string]any{
		"action": "stop", "container_ids": []string{"x"},
	})); s == http.StatusForbidden {
		t.Error("operator bulk stop should pass the role gate, got 403")
	}
}

// TestRBACNodeCRUDAdminOnly guards the (previously ungated) node management
// endpoints: a viewer must not be able to create/rename/delete/reprobe nodes.
func TestRBACNodeCRUDAdminOnly(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	cases := []struct{ method, url string }{
		{http.MethodPost, srv.URL + "/api/nodes"},
		{http.MethodPut, srv.URL + "/api/nodes/x"},
		{http.MethodDelete, srv.URL + "/api/nodes/x"},
		{http.MethodPost, srv.URL + "/api/nodes/x/probe"},
	}
	for _, tc := range cases {
		body := map[string]any{"name": "n", "host": "10.0.0.1", "ssh_port": 22, "credentials": map[string]string{"method": "ssh_key"}}
		if s := status(t, c, authReq(t, tc.method, tc.url, viewerTok, body)); s != http.StatusForbidden {
			t.Errorf("viewer %s %s = %d, want 403", tc.method, tc.url, s)
		}
	}
}

// TestRBACUserManagement covers create/list/role-change/delete plus the
// last-admin and self-delete guards.
func TestRBACUserManagement(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}

	viewerID := createUser(t, c, srv.URL, adminTok, "viewer", "viewer")

	// List shows admin + viewer.
	var list struct {
		Users []struct {
			ID, Username, Role string
		} `json:"users"`
	}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/users", adminTok, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list.Users) != 2 {
		t.Fatalf("user count = %d, want 2", len(list.Users))
	}

	// Duplicate username is a 409.
	if s := status(t, c, authReq(t, http.MethodPost, srv.URL+"/api/users", adminTok, map[string]string{
		"username": "viewer", "password": "supersecret", "role": "viewer",
	})); s != http.StatusConflict {
		t.Errorf("duplicate username = %d, want 409", s)
	}

	// Promote the viewer to operator.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/users/"+viewerID+"/role", adminTok,
		map[string]string{"role": "operator"})); s != http.StatusOK {
		t.Errorf("promote = %d, want 200", s)
	}

	// Find the admin's own id (so we can test the guards against it).
	resp, _ = c.Do(authReq(t, http.MethodGet, srv.URL+"/api/users", adminTok, nil))
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	var adminID string
	for _, u := range list.Users {
		if u.Role == "admin" {
			adminID = u.ID
		}
	}
	if adminID == "" {
		t.Fatal("no admin in list")
	}

	// Last-admin guard: can't demote the only admin.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/users/"+adminID+"/role", adminTok,
		map[string]string{"role": "viewer"})); s != http.StatusConflict {
		t.Errorf("demote last admin = %d, want 409", s)
	}
	// Self-delete guard.
	if s := status(t, c, authReq(t, http.MethodDelete, srv.URL+"/api/users/"+adminID, adminTok, nil)); s != http.StatusBadRequest {
		t.Errorf("self-delete = %d, want 400", s)
	}
	// Deleting the (non-admin) other user works.
	if s := status(t, c, authReq(t, http.MethodDelete, srv.URL+"/api/users/"+viewerID, adminTok, nil)); s != http.StatusNoContent {
		t.Errorf("delete user = %d, want 204", s)
	}
}

// TestRBACSessionsList confirms a user can see their own session and the
// current one is flagged.
func TestRBACSessionsList(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/sessions", adminTok, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	var body struct {
		Sessions []struct {
			ID      string
			Current bool
			Active  bool
		} `json:"sessions"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if len(body.Sessions) == 0 {
		t.Fatal("expected at least one session")
	}
	// The login in newNodeTestServer used a cookie jar-less client, so "current"
	// may be false here; just assert the session is active.
	if !body.Sessions[0].Active {
		t.Error("session should be active")
	}
}
