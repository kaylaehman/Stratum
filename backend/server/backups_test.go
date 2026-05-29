package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestBackupsListEmpty(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/backups", token, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/backups = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Backups []map[string]any `json:"backups"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Backups == nil {
		t.Error("backups should be an array")
	}
}

func TestStartBackupValidation(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Create a docker-capable node so we get past the capability gate to the
	// volume/dest validation. (Test nodes have no docker → 409, which also
	// proves the gate.) Unknown node => 404.
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes/nope/backups", token, map[string]string{
		"volume": "data", "dest_dir": "/mnt/backups",
	}))
	if err != nil { t.Fatalf("request: %v", err) }
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("backup on unknown node = %d, want 404", resp.StatusCode)
	}

	// A real (non-docker) node => 409 docker_not_available.
	create, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "ssh-only-bk", "host": "10.0.0.61", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_key"},
	}))
	var node struct {
		ID string `json:"id"`
	}
	json.NewDecoder(create.Body).Decode(&node)
	create.Body.Close()
	r409, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes/"+node.ID+"/backups", token, map[string]string{
		"volume": "data", "dest_dir": "/mnt/backups",
	}))
	r409.Body.Close()
	if r409.StatusCode != http.StatusConflict {
		t.Errorf("backup on non-docker node = %d, want 409", r409.StatusCode)
	}
}
