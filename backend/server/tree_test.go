package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTreeAndCapabilityGating(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Create a bare SSH node (no docker, no proxmox capability).
	var created struct {
		ID string `json:"id"`
	}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "bare", "host": "10.0.0.77", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_key"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == "" {
		t.Fatal("no node id")
	}

	// GET /api/tree returns the node with empty inventory + a seq field.
	treeResp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/tree", token, nil))
	var tree struct {
		Nodes []struct {
			ID         string `json:"id"`
			Seq        uint64 `json:"seq"`
			VMs        []any  `json:"vms"`
			Containers []any  `json:"containers"`
		} `json:"nodes"`
	}
	json.NewDecoder(treeResp.Body).Decode(&tree)
	treeResp.Body.Close()
	if len(tree.Nodes) != 1 || tree.Nodes[0].ID != created.ID {
		t.Fatalf("tree = %+v", tree)
	}
	if tree.Nodes[0].VMs == nil || tree.Nodes[0].Containers == nil {
		t.Error("vms/containers should be [] not null")
	}

	// /vms -> 422 (node is not a confirmed Proxmox node).
	vmsResp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/"+created.ID+"/vms", token, nil))
	if vmsResp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("/vms status = %d, want 422", vmsResp.StatusCode)
	}
	vmsResp.Body.Close()

	// /containers -> 422 (no docker capability).
	ctrResp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/"+created.ID+"/containers", token, nil))
	if ctrResp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("/containers status = %d, want 422", ctrResp.StatusCode)
	}
	ctrResp.Body.Close()
}
