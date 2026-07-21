package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/features"
	"github.com/KAE-Labs/stratum/backend/totp"
)

// bodyError decodes a JSON {"error": "..."} response body.
func bodyError(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var b struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&b)
	return b.Error
}

// enrollTOTP runs setup + enable for the token's user and returns the secret.
func enrollTOTP(t *testing.T, c *http.Client, base, token string) string {
	t.Helper()
	resp, err := c.Do(authReq(t, http.MethodPost, base+"/api/me/2fa/setup", token, map[string]string{}))
	if err != nil {
		t.Fatalf("2fa setup: %v", err)
	}
	var s struct {
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		t.Fatalf("decode setup: %v", err)
	}
	resp.Body.Close()
	if s.Secret == "" {
		t.Fatal("setup returned empty secret")
	}
	code, _ := totp.Code(s.Secret, time.Now())
	en, err := c.Do(authReq(t, http.MethodPost, base+"/api/me/2fa/enable", token, map[string]string{"code": code}))
	if err != nil {
		t.Fatalf("2fa enable: %v", err)
	}
	en.Body.Close()
	if en.StatusCode != http.StatusOK && en.StatusCode != http.StatusNoContent {
		t.Fatalf("2fa enable = %d, want 2xx", en.StatusCode)
	}
	return s.Secret
}

// challengeStepUp opens the step-up grace window with a fresh code.
func challengeStepUp(t *testing.T, c *http.Client, base, token, secret string) {
	t.Helper()
	code, _ := totp.Code(secret, time.Now())
	resp, err := c.Do(authReq(t, http.MethodPost, base+"/api/me/2fa/challenge", token, map[string]string{"code": code}))
	if err != nil {
		t.Fatalf("2fa challenge: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("2fa challenge = %d, want 2xx", resp.StatusCode)
	}
}

// TestStepUpFailsClosedWithoutEnrollment is the C5 regression: with the step-up
// feature enabled, an admin who has NOT enrolled TOTP must be BLOCKED from a
// destructive action (previously this silently bypassed the control). After
// enrolling and opening a step-up window, the action is allowed.
func TestStepUpFailsClosedWithoutEnrollment(t *testing.T) {
	srv, token, store := newNodeTestServerWithStore(t)
	c := &http.Client{}
	// The shared harness defaults this OFF; enable it for this test.
	if err := store.SetFeatureFlag(context.Background(), features.FlagActionStepUp, true); err != nil {
		t.Fatalf("enable feature: %v", err)
	}

	prune := srv.URL + "/api/volumes/prune-unused"

	// 1. No TOTP enrolled → fail CLOSED with an enrollment prompt (not allowed).
	resp, err := c.Do(authReq(t, http.MethodPost, prune, token, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusPreconditionRequired {
		t.Fatalf("no-TOTP prune = %d, want 428 (fail closed)", resp.StatusCode)
	}
	if e := bodyError(t, resp); e != "totp_enrollment_required" {
		t.Fatalf("error = %q, want totp_enrollment_required", e)
	}

	// 2. Enrolled but no fresh step-up confirmation → 428 2fa_required.
	secret := enrollTOTP(t, c, srv.URL, token)
	resp, err = c.Do(authReq(t, http.MethodPost, prune, token, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusPreconditionRequired {
		t.Fatalf("enrolled-no-stepup prune = %d, want 428", resp.StatusCode)
	}
	if e := bodyError(t, resp); e != "2fa_required" {
		t.Fatalf("error = %q, want 2fa_required", e)
	}

	// 3. Fresh step-up confirmation → action allowed.
	challengeStepUp(t, c, srv.URL, token, secret)
	resp, err = c.Do(authReq(t, http.MethodPost, prune, token, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stepped-up prune = %d, want 200", resp.StatusCode)
	}
}

// TestStepUpDisabledFlagAllows confirms the gate is a no-op when the feature is
// off: an admin with no TOTP can perform the action (operator escape hatch).
func TestStepUpDisabledFlagAllows(t *testing.T) {
	srv, token, store := newNodeTestServerWithStore(t)
	c := &http.Client{}
	// Explicit: feature disabled (also the harness default).
	if err := store.SetFeatureFlag(context.Background(), features.FlagActionStepUp, false); err != nil {
		t.Fatalf("disable feature: %v", err)
	}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/volumes/prune-unused", token, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("prune with feature off = %d, want 200 (no enforcement)", resp.StatusCode)
	}
}
