package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDepGraphUnknownNode(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/nope/depgraph", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("depgraph unknown node = %d, want 404", resp.StatusCode)
	}
}

func TestDepGraphNoDocker(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	createResp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "ssh-only-dg", "host": "10.0.0.51", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_key"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()
	if created.ID == "" {
		t.Fatal("node create returned no id")
	}
	resp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/"+created.ID+"/depgraph", token, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("depgraph non-docker node = %d, want 409", resp.StatusCode)
	}
}
