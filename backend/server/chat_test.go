package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestChatConfigAdminGate verifies the chat config endpoints are admin-only and
// never echo the bot token (only has_token).
func TestChatConfigAdminGate(t *testing.T) {
	srv, adminTok := newNodeTestServer(t)
	c := &http.Client{}
	createUser(t, c, srv.URL, adminTok, "viewer", "viewer")
	viewerTok := loginAs(t, c, srv.URL, "viewer")

	if s := status(t, c, authReq(t, http.MethodGet, srv.URL+"/api/chat/config", viewerTok, nil)); s != http.StatusForbidden {
		t.Errorf("viewer GET chat config = %d, want 403", s)
	}

	// Admin sets a token + allowed chats.
	if s := status(t, c, authReq(t, http.MethodPut, srv.URL+"/api/chat/config", adminTok, map[string]any{
		"allowed_chats": []int64{12345}, "token": "bot-secret-token",
	})); s != http.StatusOK {
		t.Errorf("admin set chat config = %d, want 200", s)
	}

	// GET reports has_token but never the token itself.
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/chat/config", adminTok, nil))
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if body["has_token"] != true {
		t.Errorf("has_token = %v, want true", body["has_token"])
	}
	if _, leaked := body["token"]; leaked {
		t.Error("config response leaked the bot token")
	}
	chats, _ := body["allowed_chats"].([]any)
	if len(chats) != 1 {
		t.Errorf("allowed_chats = %v, want 1 entry", body["allowed_chats"])
	}
}
