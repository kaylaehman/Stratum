package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// loadFixture reads a recorded Cloudflare API JSON response from testdata.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// dashboardConfigFixture is the recorded configurations response for a
// dashboard-managed tunnel (source="cloudflare"), pulled from testdata.
func dashboardConfigFixture(t *testing.T) []byte {
	return loadFixture(t, "cloudflare_configurations.json")
}

// locallyManagedFixture is a configurations response for a locally-managed
// tunnel: the API reports source="local" with no usable ingress.
const locallyManagedFixture = `{
  "success": true,
  "errors": [],
  "result": {
    "tunnel_id": "f70a3a6b-4f8e-4a1f-9c2d-1a2b3c4d5e6f",
    "version": 0,
    "config": null,
    "source": "local"
  }
}`

// cfServer spins up a fake Cloudflare API that serves the given body for the
// tunnel configurations endpoint and a single account for discovery.
func cfServer(t *testing.T, configBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"bad token"}]}`))
			return
		}
		switch {
		case r.URL.Path == "/accounts":
			_, _ = w.Write([]byte(`{"success":true,"errors":[],"result":[{"id":"acc-123","name":"Homelab"}]}`))
		case strings.HasSuffix(r.URL.Path, "/configurations"):
			_, _ = w.Write([]byte(configBody))
		case strings.HasSuffix(r.URL.Path, "/cfd_tunnel"):
			_, _ = w.Write([]byte(`{"success":true,"errors":[],"result":[{"id":"tun-1","name":"home"},{"id":"tun-2","name":"lab"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":1003,"message":"not found"}]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestParseCloudflareIngress(t *testing.T) {
	// Decode the recorded fixture exactly as the client would, then parse.
	env, err := decodeCFResponse(http.StatusOK, dashboardConfigFixture(t))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	var res cfTunnelConfigResult
	if err := json.Unmarshal(env.Result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.Source != "cloudflare" {
		t.Fatalf("fixture source = %q, want cloudflare (dashboard-managed)", res.Source)
	}
	rules := parseCloudflareIngress(res.Config.Ingress)

	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3 (catch-all must be skipped)", len(rules))
	}
	cases := []struct {
		host, target, path string
	}{
		{"app.example.com", "http://localhost:8080", ""},
		{"api.example.com", "http://api:3000", "/v1"},
		{"grafana.example.com", "http://198.51.100.20:3000", ""},
	}
	for i, tc := range cases {
		r := rules[i]
		if r.SourceHost != tc.host {
			t.Errorf("rules[%d].SourceHost = %q, want %q", i, r.SourceHost, tc.host)
		}
		if r.TargetURL != tc.target {
			t.Errorf("rules[%d].TargetURL = %q, want %q", i, r.TargetURL, tc.target)
		}
		if r.SourcePath != tc.path {
			t.Errorf("rules[%d].SourcePath = %q, want %q", i, r.SourcePath, tc.path)
		}
		if r.AdapterType != "cloudflare-api" {
			t.Errorf("rules[%d].AdapterType = %q, want cloudflare-api", i, r.AdapterType)
		}
		if !r.SSLEnabled {
			t.Errorf("rules[%d].SSLEnabled = false, want true", i)
		}
	}
}

// TestListRulesDashboardManaged exercises the full ListRules path against a
// fake CF API serving the recorded dashboard-managed fixture.
func TestListRulesDashboardManaged(t *testing.T) {
	srv := cfServer(t, string(dashboardConfigFixture(t)))
	cf := &CloudflareAPI{HTTP: srv.Client()}
	conn := Conn{
		Endpoint: srv.URL,
		Token:    "test-token",
		Config:   map[string]string{"account_id": "acc-123", "tunnel_id": "tun-1"},
	}
	rules, err := cf.ListRules(context.Background(), conn)
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}
}

// TestListRulesLocallyManaged verifies a locally-managed tunnel returns an
// actionable error pointing at the file-based provider, not an empty list.
func TestListRulesLocallyManaged(t *testing.T) {
	srv := cfServer(t, locallyManagedFixture)
	cf := &CloudflareAPI{HTTP: srv.Client()}
	conn := Conn{
		Endpoint: srv.URL,
		Token:    "test-token",
		Config:   map[string]string{"account_id": "acc-123", "tunnel_id": "tun-1"},
	}
	_, err := cf.ListRules(context.Background(), conn)
	if err == nil {
		t.Fatal("want error for locally-managed tunnel, got nil")
	}
	if !strings.Contains(err.Error(), "locally-managed") {
		t.Errorf("error = %q, want it to mention locally-managed", err.Error())
	}
}

// TestListRulesAccountDiscovery verifies account_id is discovered when omitted.
func TestListRulesAccountDiscovery(t *testing.T) {
	srv := cfServer(t, string(dashboardConfigFixture(t)))
	cf := &CloudflareAPI{HTTP: srv.Client()}
	conn := Conn{
		Endpoint: srv.URL,
		Token:    "test-token",
		Config:   map[string]string{"tunnel_id": "tun-1"}, // no account_id
	}
	rules, err := cf.ListRules(context.Background(), conn)
	if err != nil {
		t.Fatalf("ListRules with discovery: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}
}

func TestListRulesNoToken(t *testing.T) {
	cf := &CloudflareAPI{}
	_, err := cf.ListRules(context.Background(), Conn{Config: map[string]string{"tunnel_id": "x"}})
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Errorf("want token error, got %v", err)
	}
}

func TestListRulesNoTunnel(t *testing.T) {
	srv := cfServer(t, string(dashboardConfigFixture(t)))
	cf := &CloudflareAPI{HTTP: srv.Client()}
	conn := Conn{Endpoint: srv.URL, Token: "test-token", Config: map[string]string{"account_id": "acc-123"}}
	_, err := cf.ListRules(context.Background(), conn)
	if err == nil || !strings.Contains(err.Error(), "tunnel") {
		t.Errorf("want tunnel-not-selected error, got %v", err)
	}
}

func TestDecodeCFResponseErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"unauthorized", http.StatusUnauthorized, ``, "invalid API token"},
		{"forbidden", http.StatusForbidden, ``, "Cloudflare Tunnel → Read"},
		{"notfound", http.StatusNotFound, ``, "not found"},
		{"api-error", http.StatusOK, `{"success":false,"errors":[{"code":1,"message":"boom"}]}`, "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeCFResponse(tc.status, []byte(tc.body))
			if err == nil {
				t.Fatalf("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want contains %q", err.Error(), tc.want)
			}
		})
	}
}

func TestListTunnels(t *testing.T) {
	srv := cfServer(t, string(dashboardConfigFixture(t)))
	cf := &CloudflareAPI{HTTP: srv.Client()}
	conn := Conn{Endpoint: srv.URL, Token: "test-token", Config: map[string]string{"account_id": "acc-123"}}
	tunnels, err := cf.ListTunnels(context.Background(), conn)
	if err != nil {
		t.Fatalf("ListTunnels: %v", err)
	}
	if len(tunnels) != 2 || tunnels[0].ID != "tun-1" || tunnels[1].Name != "lab" {
		t.Errorf("ListTunnels = %+v, want 2 tunnels [tun-1 home, tun-2 lab]", tunnels)
	}
}

func TestListAccounts(t *testing.T) {
	srv := cfServer(t, string(dashboardConfigFixture(t)))
	cf := &CloudflareAPI{HTTP: srv.Client()}
	conn := Conn{Endpoint: srv.URL, Token: "test-token"}
	accounts, err := cf.ListAccounts(context.Background(), conn)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != "acc-123" {
		t.Errorf("ListAccounts = %+v, want [acc-123 Homelab]", accounts)
	}
}

// TestListRulesRejectsPathInjection guards H1: a tunnel/account id containing
// path separators must be rejected before any request is built, so it can't
// smuggle path segments into the Cloudflare API URL.
func TestListRulesRejectsPathInjection(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cf := &CloudflareAPI{HTTP: srv.Client()}
	conn := Conn{
		Endpoint: srv.URL,
		Token:    "test-token",
		Config:   map[string]string{"account_id": "acc-123", "tunnel_id": "../../other/configurations"},
	}
	_, err := cf.ListRules(context.Background(), conn)
	if err == nil || !strings.Contains(err.Error(), "invalid tunnel id") {
		t.Errorf("want invalid-tunnel-id error, got %v", err)
	}
	if hit {
		t.Error("a request was sent despite an invalid tunnel id")
	}
}

func TestCloudflareAPIMutationsUnsupported(t *testing.T) {
	cf := &CloudflareAPI{}
	if _, err := cf.CreateRule(context.Background(), Conn{}, Rule{}); err != ErrUnsupported {
		t.Errorf("CreateRule = %v, want ErrUnsupported", err)
	}
	if err := cf.UpdateRule(context.Background(), Conn{}, "", Rule{}); err != ErrUnsupported {
		t.Errorf("UpdateRule = %v, want ErrUnsupported", err)
	}
	if err := cf.DeleteRule(context.Background(), Conn{}, ""); err != ErrUnsupported {
		t.Errorf("DeleteRule = %v, want ErrUnsupported", err)
	}
}

// TestCloudflareAPINotImageDetected guards the opt-in invariant: the API
// adapter must never be selected by image detection (it has no ImagePatterns),
// so existing cloudflared/nginx/traefik/caddy detection is untouched.
func TestCloudflareAPINotImageDetected(t *testing.T) {
	if a := DetectByImages([]string{"cloudflare/cloudflared:latest"}); a == nil || a.Name() != "cloudflared" {
		t.Errorf("cloudflared image still detects as %v, want file-based cloudflared", a)
	}
	cf := &CloudflareAPI{}
	if len(cf.ImagePatterns()) != 0 {
		t.Errorf("CloudflareAPI.ImagePatterns = %v, want empty (opt-in only)", cf.ImagePatterns())
	}
}
