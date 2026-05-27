package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestBookmarksLifecycle(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Empty initially.
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/bookmarks", token, nil))
	var list struct {
		Bookmarks []map[string]any `json:"bookmarks"`
	}
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if list.Bookmarks == nil {
		t.Fatal("bookmarks should be an array")
	}

	// Create.
	createResp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/bookmarks", token, map[string]string{
		"label": "Plex", "resource_type": "container", "resource_ref": "c1",
	}))
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d, want 201", createResp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()
	if created.ID == "" {
		t.Fatal("create returned no id")
	}

	// Missing required field => 400.
	badResp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/bookmarks", token, map[string]string{"label": "x"}))
	badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Errorf("incomplete create = %d, want 400", badResp.StatusCode)
	}

	// Delete.
	delResp, _ := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/bookmarks/"+created.ID, token, nil))
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", delResp.StatusCode)
	}

	// Delete unknown => 404.
	del2, _ := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/bookmarks/nope", token, nil))
	del2.Body.Close()
	if del2.StatusCode != http.StatusNotFound {
		t.Errorf("delete unknown = %d, want 404", del2.StatusCode)
	}
}
