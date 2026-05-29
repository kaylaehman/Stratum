package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTwoFAStatusDefaultDisabled(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/me/2fa", token, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/me/2fa = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Enabled {
		t.Error("2FA should be disabled by default")
	}
}

func TestTwoFASetupReturnsSecretAndRecovery(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/me/2fa/setup", token, nil))
	if err != nil { t.Fatalf("request: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Secret          string   `json:"secret"`
		ProvisioningURI string   `json:"provisioning_uri"`
		RecoveryCodes   []string `json:"recovery_codes"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Secret == "" || len(body.RecoveryCodes) != 8 || body.ProvisioningURI == "" {
		t.Errorf("setup result incomplete: secret=%q codes=%d uri=%q", body.Secret, len(body.RecoveryCodes), body.ProvisioningURI)
	}

	// Enabling with a bogus code is rejected.
	en, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/me/2fa/enable", token, map[string]string{"code": "000000"}))
	en.Body.Close()
	if en.StatusCode != http.StatusBadRequest {
		t.Errorf("enable bad code = %d, want 400", en.StatusCode)
	}
}
