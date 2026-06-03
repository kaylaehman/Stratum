package proxy

import (
	"context"
	"encoding/json"
	"errors"
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
	// Create is now supported (see TestCreateRule*); Update/Delete remain
	// intentionally unsupported to bound the blast radius on live infra.
	if err := cf.UpdateRule(context.Background(), Conn{}, "", Rule{}); err != ErrUnsupported {
		t.Errorf("UpdateRule = %v, want ErrUnsupported", err)
	}
	if err := cf.DeleteRule(context.Background(), Conn{}, ""); err != ErrUnsupported {
		t.Errorf("DeleteRule = %v, want ErrUnsupported", err)
	}
}

func TestCloudflareAPICapabilitiesIncludeCreate(t *testing.T) {
	caps := (&CloudflareAPI{}).Capabilities()
	if !caps.List || !caps.Create {
		t.Errorf("Capabilities = %+v, want List+Create", caps)
	}
	if caps.Update || caps.Delete {
		t.Errorf("Capabilities = %+v, want Update/Delete false", caps)
	}
}

func TestInsertIngress(t *testing.T) {
	catchAll := cfIngress{Service: "http_status:404"}
	t.Run("inserts before catch-all", func(t *testing.T) {
		existing := []cfIngress{{Hostname: "a.example.com", Service: "http://a:1"}, catchAll}
		got, err := insertIngress(existing, cfIngress{Hostname: "b.example.com", Service: "http://b:2"})
		if err != nil {
			t.Fatalf("insertIngress: %v", err)
		}
		if len(got) != 3 || got[0].Hostname != "a.example.com" || got[1].Hostname != "b.example.com" || got[2].Hostname != "" {
			t.Fatalf("order wrong: %+v", got)
		}
	})
	t.Run("rejects duplicate host+path", func(t *testing.T) {
		existing := []cfIngress{{Hostname: "a.example.com", Service: "http://a:1"}, catchAll}
		if _, err := insertIngress(existing, cfIngress{Hostname: "a.example.com", Service: "http://x:9"}); err == nil {
			t.Fatal("want duplicate error, got nil")
		}
	})
	t.Run("appends catch-all when none present", func(t *testing.T) {
		got, err := insertIngress(nil, cfIngress{Hostname: "b.example.com", Service: "http://b:2"})
		if err != nil {
			t.Fatalf("insertIngress: %v", err)
		}
		if len(got) != 2 || got[0].Hostname != "b.example.com" || got[1].Hostname != "" || got[1].Service != "http_status:404" {
			t.Fatalf("want [route, catch-all], got %+v", got)
		}
	})
	t.Run("same host different path is allowed", func(t *testing.T) {
		existing := []cfIngress{{Hostname: "a.example.com", Path: "/v1", Service: "http://a:1"}, catchAll}
		got, err := insertIngress(existing, cfIngress{Hostname: "a.example.com", Path: "/v2", Service: "http://a:2"})
		if err != nil {
			t.Fatalf("insertIngress: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("want 3 entries, got %+v", got)
		}
	})
}

func TestZoneCandidates(t *testing.T) {
	got := zoneCandidates("jellyfin.kaylas.systems")
	want := []string{"jellyfin.kaylas.systems", "kaylas.systems"}
	if len(got) != len(want) {
		t.Fatalf("zoneCandidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("zoneCandidates = %v, want %v", got, want)
		}
	}
}

func TestValidateProxyHostname(t *testing.T) {
	ok := []string{"jellyfin.kaylas.systems", "a.b.c.example.com"}
	for _, h := range ok {
		if err := validateProxyHostname(h); err != nil {
			t.Errorf("validateProxyHostname(%q) = %v, want nil", h, err)
		}
	}
	bad := []string{"", "nodot", "http://x.com", "x.com:8080", "*.example.com", "x..com"}
	for _, h := range bad {
		if err := validateProxyHostname(h); err == nil {
			t.Errorf("validateProxyHostname(%q) = nil, want error", h)
		}
	}
}

func TestValidateProxyService(t *testing.T) {
	if err := validateProxyService("http://192.168.20.9:8096"); err != nil {
		t.Errorf("valid service rejected: %v", err)
	}
	for _, s := range []string{"", "ssh://x:22", "ftp://x", "http_status:404", "://nohost"} {
		if err := validateProxyService(s); err == nil {
			t.Errorf("validateProxyService(%q) = nil, want error", s)
		}
	}
}

// cfWriteServer is a fake Cloudflare API that records the PUT config + DNS POST
// and serves discovery/config/zone/record reads, so CreateRule can be exercised
// end-to-end without a live API.
type cfWriteServer struct {
	srv        *httptest.Server
	putBody    map[string]any // captured PUT .../configurations body
	dnsPosted  *cfDNSRecord   // captured POST .../dns_records body
	dnsRecords []cfDNSRecord  // pre-seeded existing records (by exact name match)
	zones      []cfZone       // zones the token can "see"
}

func newCFWriteServer(t *testing.T, configBody string) *cfWriteServer {
	t.Helper()
	ws := &cfWriteServer{zones: []cfZone{{ID: "zone-1", Name: "example.com"}}}
	ws.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/accounts":
			_, _ = w.Write([]byte(`{"success":true,"errors":[],"result":[{"id":"acc-123","name":"Homelab"}]}`))
		case strings.HasSuffix(r.URL.Path, "/configurations") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(configBody))
		case strings.HasSuffix(r.URL.Path, "/configurations") && r.Method == http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&ws.putBody)
			_, _ = w.Write([]byte(`{"success":true,"errors":[],"result":{}}`))
		case r.URL.Path == "/zones":
			name := r.URL.Query().Get("name")
			var match []cfZone
			for _, z := range ws.zones {
				if z.Name == name {
					match = append(match, z)
				}
			}
			b, _ := json.Marshal(map[string]any{"success": true, "errors": []any{}, "result": match})
			_, _ = w.Write(b)
		case strings.HasSuffix(r.URL.Path, "/dns_records") && r.Method == http.MethodGet:
			name := r.URL.Query().Get("name")
			var match []cfDNSRecord
			for _, rec := range ws.dnsRecords {
				if rec.Name == name {
					match = append(match, rec)
				}
			}
			b, _ := json.Marshal(map[string]any{"success": true, "errors": []any{}, "result": match})
			_, _ = w.Write(b)
		case strings.HasSuffix(r.URL.Path, "/dns_records") && r.Method == http.MethodPost:
			var rec cfDNSRecord
			_ = json.NewDecoder(r.Body).Decode(&rec)
			ws.dnsPosted = &rec
			_, _ = w.Write([]byte(`{"success":true,"errors":[],"result":{"id":"rec-1"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":1003,"message":"not found"}]}`))
		}
	}))
	t.Cleanup(ws.srv.Close)
	return ws
}

func (ws *cfWriteServer) conn() Conn {
	return Conn{
		Endpoint: ws.srv.URL,
		Token:    "test-token",
		Config:   map[string]string{"account_id": "acc-123", "tunnel_id": "tun-1"},
	}
}

func TestCreateRuleInsertsIngressAndDNS(t *testing.T) {
	ws := newCFWriteServer(t, string(dashboardConfigFixture(t)))
	cf := &CloudflareAPI{HTTP: ws.srv.Client()}
	out, err := cf.CreateRule(context.Background(), ws.conn(), Rule{
		SourceHost: "media.example.com",
		TargetURL:  "http://192.168.20.9:8096",
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if out.SourceHost != "media.example.com" || !out.SSLEnabled {
		t.Errorf("returned rule = %+v", out)
	}
	// PUT body must contain the new hostname, before the catch-all.
	cfg, _ := ws.putBody["config"].(map[string]any)
	ingress, _ := cfg["ingress"].([]any)
	if len(ingress) == 0 {
		t.Fatalf("no ingress in PUT body: %+v", ws.putBody)
	}
	last, _ := ingress[len(ingress)-1].(map[string]any)
	if h, _ := last["hostname"].(string); h != "" {
		t.Errorf("last ingress entry must be the catch-all, got hostname=%q", h)
	}
	found := false
	for _, e := range ingress {
		m, _ := e.(map[string]any)
		if m["hostname"] == "media.example.com" {
			found = true
		}
	}
	if !found {
		t.Errorf("new hostname not in PUT ingress: %+v", ingress)
	}
	// DNS CNAME must be created, proxied, pointing at the tunnel.
	if ws.dnsPosted == nil {
		t.Fatal("no DNS record POSTed")
	}
	if ws.dnsPosted.Type != "CNAME" || !ws.dnsPosted.Proxied || !strings.HasSuffix(ws.dnsPosted.Content, ".cfargotunnel.com") {
		t.Errorf("DNS record = %+v, want proxied CNAME to *.cfargotunnel.com", *ws.dnsPosted)
	}
}

func TestCreateRuleSkipsDNSWhenOptedOut(t *testing.T) {
	ws := newCFWriteServer(t, string(dashboardConfigFixture(t)))
	cf := &CloudflareAPI{HTTP: ws.srv.Client()}
	conn := ws.conn()
	conn.Config["create_dns"] = "false"
	if _, err := cf.CreateRule(context.Background(), conn, Rule{SourceHost: "media.example.com", TargetURL: "http://10.0.0.1:80"}); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if ws.putBody == nil {
		t.Fatal("ingress PUT did not happen")
	}
	if ws.dnsPosted != nil {
		t.Errorf("DNS record POSTed despite create_dns=false: %+v", *ws.dnsPosted)
	}
}

func TestCreateRuleExistingTunnelDNSIsIdempotent(t *testing.T) {
	ws := newCFWriteServer(t, string(dashboardConfigFixture(t)))
	ws.dnsRecords = []cfDNSRecord{{Type: "CNAME", Name: "media.example.com", Content: "tun-1.cfargotunnel.com", Proxied: true}}
	cf := &CloudflareAPI{HTTP: ws.srv.Client()}
	if _, err := cf.CreateRule(context.Background(), ws.conn(), Rule{SourceHost: "media.example.com", TargetURL: "http://10.0.0.1:80"}); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if ws.dnsPosted != nil {
		t.Errorf("created a DNS record when a tunnel CNAME already existed: %+v", *ws.dnsPosted)
	}
}

func TestCreateRuleConflictingDNSWarns(t *testing.T) {
	ws := newCFWriteServer(t, string(dashboardConfigFixture(t)))
	ws.dnsRecords = []cfDNSRecord{{Type: "A", Name: "media.example.com", Content: "203.0.113.5"}}
	cf := &CloudflareAPI{HTTP: ws.srv.Client()}
	_, err := cf.CreateRule(context.Background(), ws.conn(), Rule{SourceHost: "media.example.com", TargetURL: "http://10.0.0.1:80"})
	if !errors.Is(err, ErrDNSWarning) {
		t.Fatalf("err = %v, want ErrDNSWarning (ingress succeeded, DNS conflict)", err)
	}
	if ws.putBody == nil {
		t.Error("ingress should still have been written before the DNS warning")
	}
}

func TestCreateRuleDifferentTunnelDNSWarns(t *testing.T) {
	ws := newCFWriteServer(t, string(dashboardConfigFixture(t)))
	// An existing CNAME points the hostname at a DIFFERENT tunnel.
	ws.dnsRecords = []cfDNSRecord{{Type: "CNAME", Name: "media.example.com", Content: "other-tunnel.cfargotunnel.com", Proxied: true}}
	cf := &CloudflareAPI{HTTP: ws.srv.Client()}
	_, err := cf.CreateRule(context.Background(), ws.conn(), Rule{SourceHost: "media.example.com", TargetURL: "http://10.0.0.1:80"})
	if !errors.Is(err, ErrDNSWarning) {
		t.Fatalf("err = %v, want ErrDNSWarning for a different-tunnel CNAME", err)
	}
	if !strings.Contains(err.Error(), "different tunnel") {
		t.Errorf("warning = %q, want it to mention a different tunnel", err.Error())
	}
	if ws.dnsPosted != nil {
		t.Error("must not create/overwrite a record that points at another tunnel")
	}
	if ws.putBody == nil {
		t.Error("the ingress route should still have been written")
	}
}

func TestCreateRuleRejectsDuplicateHostname(t *testing.T) {
	ws := newCFWriteServer(t, string(dashboardConfigFixture(t)))
	cf := &CloudflareAPI{HTTP: ws.srv.Client()}
	// app.example.com is already in the fixture ingress.
	_, err := cf.CreateRule(context.Background(), ws.conn(), Rule{SourceHost: "app.example.com", TargetURL: "http://10.0.0.1:80"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want duplicate-route error", err)
	}
	if ws.putBody != nil {
		t.Error("config was PUT despite a duplicate hostname")
	}
}

func TestCreateRuleRejectsLocallyManaged(t *testing.T) {
	ws := newCFWriteServer(t, locallyManagedFixture)
	cf := &CloudflareAPI{HTTP: ws.srv.Client()}
	_, err := cf.CreateRule(context.Background(), ws.conn(), Rule{SourceHost: "media.example.com", TargetURL: "http://10.0.0.1:80"})
	if err == nil || !strings.Contains(err.Error(), "locally-managed") {
		t.Fatalf("err = %v, want locally-managed rejection", err)
	}
	if ws.putBody != nil {
		t.Error("config was PUT for a locally-managed tunnel")
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
