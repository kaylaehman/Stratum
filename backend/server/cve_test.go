package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
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

// TestCVEDetailReturnsVulns is the regression test for the bug where
// GET /api/security/cve/{digest} returned an empty vulns array even when
// cve_results rows existed for the digest. The root cause was that old scan
// records were stored with image_digest="" (before resolveDigest was fixed to
// never return ""); the frontend's useCVEDetail hook guards on digest.length>0
// so those rows permanently showed non-zero counts with an empty detail panel.
//
// This test seeds one image_scan + two cve_result rows under a real digest and
// asserts the detail endpoint returns them both.
func TestCVEDetailReturnsVulns(t *testing.T) {
	srv, token, store := newNodeTestServerWithStore(t)
	ctx := context.Background()

	digest := "sha256:deadbeefdeadbeefdeadbeefdeadbeef"

	// Seed an image scan summary.
	if err := store.UpsertImageScan(ctx, appdb.ImageScanRow{
		ImageDigest: digest,
		Image:       "nginx:latest",
		ScannedAt:   time.Now(),
		Critical:    1,
		High:        1,
	}); err != nil {
		t.Fatalf("seed image scan: %v", err)
	}

	// Seed two vulnerability rows under the same digest.
	if err := store.ReplaceCVEResults(ctx, digest, []appdb.CVEResultRow{
		{ImageDigest: digest, CVEID: "CVE-2024-0001", Severity: "CRITICAL", Package: "libc6", InstalledVersion: "2.36-9", FixedVersion: "2.36-10", Title: "libc overflow"},
		{ImageDigest: digest, CVEID: "CVE-2024-0002", Severity: "HIGH", Package: "openssl", InstalledVersion: "3.0.11", FixedVersion: "", Title: "openssl issue"},
	}); err != nil {
		t.Fatalf("seed cve results: %v", err)
	}

	c := &http.Client{}
	url := fmt.Sprintf("%s/api/security/cve/%s", srv.URL, digest)
	resp, err := c.Do(authReq(t, http.MethodGet, url, token, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CVEDetail = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Vulns []struct {
			CVEID    string `json:"cve_id"`
			Severity string `json:"severity"`
		} `json:"vulns"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Vulns) != 2 {
		t.Fatalf("got %d vulns, want 2", len(body.Vulns))
	}
}

// TestCVEDetailEmptyDigestNotFound verifies that querying an unknown/empty
// digest returns an empty vulns array (not an error), matching the
// frontend expectation for clean (no-CVE) images.
func TestCVEDetailEmptyDigestNotFound(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/security/cve/sha256%3Anotfound", token, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CVEDetail unknown digest = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Vulns []any `json:"vulns"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Vulns) != 0 {
		t.Fatalf("unknown digest: got %d vulns, want 0", len(body.Vulns))
	}
}
