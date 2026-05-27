package proxy

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDetectByImagesSpecificityWins(t *testing.T) {
	// NPM image contains "nginx" but the more specific NPM pattern must win.
	if a := DetectByImages([]string{"jc21/nginx-proxy-manager:latest"}); a == nil || a.Name() != "nginx-proxy-manager" {
		t.Errorf("NPM image detected as %v, want nginx-proxy-manager", a)
	}
	if a := DetectByImages([]string{"traefik:v3"}); a == nil || a.Name() != "traefik" {
		t.Errorf("traefik detected as %v", a)
	}
	if a := DetectByImages([]string{"library/nginx:1.27"}); a == nil || a.Name() != "nginx" {
		t.Errorf("nginx detected as %v", a)
	}
	if a := DetectByImages([]string{"caddy:2"}); a == nil || a.Name() != "caddy" {
		t.Errorf("caddy detected as %v", a)
	}
	if a := DetectByImages([]string{"postgres:16", "redis:7"}); a != nil {
		t.Errorf("no proxy expected, got %v", a.Name())
	}
}

func TestSupportedToolsCoversAll(t *testing.T) {
	names := map[string]bool{}
	for _, ti := range SupportedTools() {
		names[ti.Name] = true
	}
	for _, want := range []string{"traefik", "nginx-proxy-manager", "caddy", "cloudflared", "haproxy", "nginx"} {
		if !names[want] {
			t.Errorf("supported tools missing %q", want)
		}
	}
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTraefikListRules(t *testing.T) {
	body := `[{"name":"web@docker","rule":"Host(` + "`app.example.com`" + `) && PathPrefix(` + "`/`" + `)","service":"app","tls":{"certResolver":"le"}},
	          {"name":"api@docker","rule":"Host(` + "`api.example.com`" + `)","service":"api"}]`
	tr := &Traefik{HTTP: &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/api/http/routers") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}

	rules, err := tr.ListRules(context.Background(), Conn{Endpoint: "http://traefik.lan:8080"})
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if rules[0].SourceHost != "app.example.com" || !rules[0].SSLEnabled {
		t.Errorf("rule0 = %+v", rules[0])
	}
	if rules[1].SourceHost != "api.example.com" || rules[1].SSLEnabled {
		t.Errorf("rule1 = %+v", rules[1])
	}
}

func TestTraefikListRulesNoEndpoint(t *testing.T) {
	tr := &Traefik{HTTP: http.DefaultClient}
	if _, err := tr.ListRules(context.Background(), Conn{}); err == nil {
		t.Error("want error when endpoint not configured")
	}
}

func TestUnsupportedMutation(t *testing.T) {
	tr := &Traefik{HTTP: http.DefaultClient}
	if _, err := tr.CreateRule(context.Background(), Conn{}, Rule{}); err != ErrUnsupported {
		t.Errorf("CreateRule err = %v, want ErrUnsupported", err)
	}
}
