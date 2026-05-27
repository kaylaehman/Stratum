package dns

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDetectByImages(t *testing.T) {
	if a := DetectByImages([]string{"adguard/adguardhome:latest"}); a == nil || a.Name() != "adguardhome" {
		t.Errorf("adguard detected as %v", a)
	}
	if a := DetectByImages([]string{"pihole/pihole:2024"}); a == nil || a.Name() != "pihole" {
		t.Errorf("pihole detected as %v", a)
	}
	if a := DetectByImages([]string{"coredns/coredns"}); a == nil || a.Name() != "coredns" {
		t.Errorf("coredns detected as %v", a)
	}
	if a := DetectByImages([]string{"nginx:1.27"}); a != nil {
		t.Errorf("no DNS tool expected, got %v", a.Name())
	}
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAdGuardListRecords(t *testing.T) {
	body := `[{"domain":"app.lan","answer":"192.168.1.10"},
	          {"domain":"v6.lan","answer":"fd00::1"},
	          {"domain":"alias.lan","answer":"app.lan"}]`
	var gotAuth string
	ag := &AdGuard{HTTP: &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		if !strings.HasSuffix(r.URL.Path, "/control/rewrite/list") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}

	recs, err := ag.ListRecords(context.Background(), Conn{Endpoint: "http://adguard.lan", Token: "tkn"})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if gotAuth != "Bearer tkn" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3", len(recs))
	}
	if recs[0].Type != "A" || recs[1].Type != "AAAA" || recs[2].Type != "CNAME" {
		t.Errorf("type inference wrong: %q %q %q", recs[0].Type, recs[1].Type, recs[2].Type)
	}
	if recs[0].Name != "app.lan" || recs[0].Value != "192.168.1.10" {
		t.Errorf("rec0 = %+v", recs[0])
	}
}

func TestAdGuardNoEndpoint(t *testing.T) {
	ag := &AdGuard{HTTP: http.DefaultClient}
	if _, err := ag.ListRecords(context.Background(), Conn{}); err == nil {
		t.Error("want error when endpoint not configured")
	}
}
