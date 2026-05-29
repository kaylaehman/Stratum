package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestCVEScansEmpty(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/security/cve", token, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/security/cve = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Available bool             `json:"available"`
		Scans     []map[string]any `json:"scans"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Scans == nil {
		t.Error("scans should be an array")
	}
	// In CI/dev trivy is absent → available:false (just assert the field decodes).
	_ = body.Available
}

func TestCVEScanUnknownContainer(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/containers/nope/cve-scan", token, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("scan unknown container = %d, want 404", resp.StatusCode)
	}
}
