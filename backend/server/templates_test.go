package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTemplateCRUDAndRender(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Empty list.
	lr, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/templates", token, nil))
	var list struct {
		Templates []map[string]any `json:"templates"`
	}
	json.NewDecoder(lr.Body).Decode(&list)
	lr.Body.Close()
	if list.Templates == nil {
		t.Fatal("templates should be an array")
	}

	// Missing compose => 400.
	bad, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/templates", token, map[string]any{"name": "x"}))
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("incomplete create = %d, want 400", bad.StatusCode)
	}

	// Create with a variable.
	cr, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/templates", token, map[string]any{
		"name": "nginx", "compose_yaml": "image: nginx:{{TAG}}",
		"variables": []map[string]string{{"name": "TAG", "default": "latest"}},
	}))
	if cr.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d, want 201", cr.StatusCode)
	}
	var created struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	json.NewDecoder(cr.Body).Decode(&created)
	cr.Body.Close()
	if created.ID == "" || created.Version != 1 {
		t.Fatalf("created = %+v", created)
	}

	// Render with default.
	rr, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/templates/"+created.ID+"/render", token, map[string]any{
		"variables": map[string]string{},
	}))
	var rendered struct {
		Rendered   string   `json:"rendered"`
		Unresolved []string `json:"unresolved"`
	}
	json.NewDecoder(rr.Body).Decode(&rendered)
	rr.Body.Close()
	if rendered.Rendered != "image: nginx:latest" || len(rendered.Unresolved) != 0 {
		t.Errorf("render = %+v", rendered)
	}

	// Update bumps version to 2.
	ur, _ := c.Do(authReq(t, http.MethodPut, srv.URL+"/api/templates/"+created.ID, token, map[string]any{
		"name": "nginx", "compose_yaml": "image: nginx:{{TAG}}-alpine",
		"variables": []map[string]string{{"name": "TAG", "default": "stable"}},
	}))
	var updated struct {
		Version int `json:"version"`
	}
	json.NewDecoder(ur.Body).Decode(&updated)
	ur.Body.Close()
	if updated.Version != 2 {
		t.Errorf("update version = %d, want 2", updated.Version)
	}

	// Get returns version history (2 versions).
	gr, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/templates/"+created.ID, token, nil))
	var detail struct {
		Versions []map[string]any `json:"versions"`
	}
	json.NewDecoder(gr.Body).Decode(&detail)
	gr.Body.Close()
	if len(detail.Versions) != 2 {
		t.Errorf("version history = %d, want 2", len(detail.Versions))
	}

	// Delete.
	dr, _ := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/templates/"+created.ID, token, nil))
	dr.Body.Close()
	if dr.StatusCode != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", dr.StatusCode)
	}
}
