package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSecretsVaultFlow(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Create a group.
	gr, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/secret-groups", token, map[string]string{
		"name": "plex-stack", "description": "Plex env",
	}))
	if gr.StatusCode != http.StatusCreated {
		t.Fatalf("create group = %d, want 201", gr.StatusCode)
	}
	var group struct {
		ID string `json:"id"`
	}
	json.NewDecoder(gr.Body).Decode(&group)
	gr.Body.Close()

	// Set a secret.
	sr, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/secret-groups/"+group.ID+"/secrets", token, map[string]string{
		"key": "API_TOKEN", "value": "topsecret",
	}))
	sr.Body.Close()
	if sr.StatusCode != http.StatusNoContent {
		t.Fatalf("set secret = %d, want 204", sr.StatusCode)
	}

	// List: the group + key name appear, but NOT the value.
	lr, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/secrets", token, nil))
	raw, _ := io.ReadAll(lr.Body)
	lr.Body.Close()
	body := string(raw)
	if strings.Contains(body, "topsecret") {
		t.Fatal("secret value leaked into the list response")
	}
	if !strings.Contains(body, "API_TOKEN") {
		t.Error("key name should appear in the list")
	}

	// Find the secret id via the parsed list.
	var list struct {
		Groups []struct {
			Secrets []struct {
				ID  string `json:"id"`
				Key string `json:"key"`
			} `json:"secrets"`
		} `json:"groups"`
	}
	json.Unmarshal([]byte(body), &list)
	var secretID string
	for _, g := range list.Groups {
		for _, s := range g.Secrets {
			if s.Key == "API_TOKEN" {
				secretID = s.ID
			}
		}
	}
	if secretID == "" {
		t.Fatal("could not find secret id")
	}

	// Reveal returns the plaintext.
	rr, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/secrets/"+secretID+"/reveal", token, nil))
	var revealed struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	json.NewDecoder(rr.Body).Decode(&revealed)
	rr.Body.Close()
	if revealed.Value != "topsecret" || revealed.Key != "API_TOKEN" {
		t.Errorf("reveal = %+v", revealed)
	}

	// Import from .env adds more.
	ir, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/secret-groups/"+group.ID+"/import", token, map[string]string{
		"env": "FOO=1\nBAR=2\n# comment\n",
	}))
	var imp struct {
		Imported int `json:"imported"`
	}
	json.NewDecoder(ir.Body).Decode(&imp)
	ir.Body.Close()
	if imp.Imported != 2 {
		t.Errorf("imported = %d, want 2", imp.Imported)
	}

	// Delete the group.
	dr, _ := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/secret-groups/"+group.ID, token, nil))
	dr.Body.Close()
	if dr.StatusCode != http.StatusNoContent {
		t.Errorf("delete group = %d, want 204", dr.StatusCode)
	}
}
