package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestWOLConfigAndWake(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Create a node to attach WOL config to.
	createResp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "wol-host", "host": "10.0.0.77", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_key"},
	}))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()
	if created.ID == "" {
		t.Fatal("no node id")
	}

	// No config yet => GET 404, wake 409.
	g404, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/"+created.ID+"/wol", token, nil))
	g404.Body.Close()
	if g404.StatusCode != http.StatusNotFound {
		t.Errorf("GET wol (none) = %d, want 404", g404.StatusCode)
	}
	wake409, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes/"+created.ID+"/wake", token, nil))
	wake409.Body.Close()
	if wake409.StatusCode != http.StatusConflict {
		t.Errorf("wake (unconfigured) = %d, want 409", wake409.StatusCode)
	}

	// Invalid MAC => 400.
	bad, _ := c.Do(authReq(t, http.MethodPut, srv.URL+"/api/nodes/"+created.ID+"/wol", token, map[string]any{"mac": "nope"}))
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("set wol (bad mac) = %d, want 400", bad.StatusCode)
	}

	// Valid set => 204, then GET returns it.
	set, _ := c.Do(authReq(t, http.MethodPut, srv.URL+"/api/nodes/"+created.ID+"/wol", token, map[string]any{
		"mac": "aa:bb:cc:dd:ee:ff", "broadcast": "192.168.1.255", "port": 9,
	}))
	set.Body.Close()
	if set.StatusCode != http.StatusNoContent {
		t.Fatalf("set wol = %d, want 204", set.StatusCode)
	}
	get, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/"+created.ID+"/wol", token, nil))
	defer get.Body.Close()
	var cfg struct {
		MAC       string `json:"mac"`
		Broadcast string `json:"broadcast"`
		Port      int    `json:"port"`
	}
	json.NewDecoder(get.Body).Decode(&cfg)
	if cfg.MAC != "aa:bb:cc:dd:ee:ff" || cfg.Broadcast != "192.168.1.255" || cfg.Port != 9 {
		t.Errorf("got config %+v", cfg)
	}
}
