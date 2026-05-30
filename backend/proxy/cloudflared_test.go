package proxy

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
)

// sampleConfig is a realistic cloudflared config.yml with several ingress
// rules plus the mandatory catch-all.
const sampleConfig = `
tunnel: abc123
credentials-file: /root/.cloudflared/abc123.json

ingress:
  - hostname: jellyfin.example.com
    service: http://jellyfin:8096
  - hostname: grafana.example.com
    service: http://localhost:3000
  - hostname: auth.example.com
    service: http://authelia:9091
    path: /
  - service: http_status:404
`

func TestParseCloudflaredConfig(t *testing.T) {
	rules, err := parseCloudflaredConfig([]byte(sampleConfig))
	if err != nil {
		t.Fatalf("parseCloudflaredConfig: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3 (catch-all must be skipped)", len(rules))
	}

	cases := []struct {
		wantHost   string
		wantTarget string
		wantPath   string
	}{
		{"jellyfin.example.com", "http://jellyfin:8096", ""},
		{"grafana.example.com", "http://localhost:3000", ""},
		{"auth.example.com", "http://authelia:9091", "/"},
	}
	for i, tc := range cases {
		r := rules[i]
		if r.SourceHost != tc.wantHost {
			t.Errorf("rules[%d].SourceHost = %q, want %q", i, r.SourceHost, tc.wantHost)
		}
		if r.TargetURL != tc.wantTarget {
			t.Errorf("rules[%d].TargetURL = %q, want %q", i, r.TargetURL, tc.wantTarget)
		}
		if r.SourcePath != tc.wantPath {
			t.Errorf("rules[%d].SourcePath = %q, want %q", i, r.SourcePath, tc.wantPath)
		}
		if r.AdapterType != "cloudflared" {
			t.Errorf("rules[%d].AdapterType = %q, want cloudflared", i, r.AdapterType)
		}
		if !r.SSLEnabled {
			t.Errorf("rules[%d].SSLEnabled = false, want true", i)
		}
	}
}

func TestParseCloudflaredConfigDashboardManaged(t *testing.T) {
	// Remotely-managed tunnel: only tunnel+credentials, no ingress block.
	cfg := `
tunnel: abc123
credentials-file: /root/.cloudflared/abc123.json
`
	rules, err := parseCloudflaredConfig([]byte(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("got %d rules for dashboard-managed tunnel, want 0", len(rules))
	}
}

func TestParseCloudflaredConfigOnlyCatchAll(t *testing.T) {
	// Config with only a catch-all entry (no named hostnames).
	cfg := `
ingress:
  - service: http_status:404
`
	rules, err := parseCloudflaredConfig([]byte(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("got %d rules, want 0 (only catch-all present)", len(rules))
	}
}

func TestCloudflaredListRules(t *testing.T) {
	cf := &Cloudflared{}
	conn := Conn{
		ReadFile: func(_ context.Context, p string) (io.ReadCloser, error) {
			if p == "/etc/cloudflared/config.yml" {
				return io.NopCloser(strings.NewReader(sampleConfig)), nil
			}
			return nil, io.ErrUnexpectedEOF
		},
	}
	rules, err := cf.ListRules(context.Background(), conn)
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}
}

func TestCloudflaredListRulesNoReadFile(t *testing.T) {
	cf := &Cloudflared{}
	if _, err := cf.ListRules(context.Background(), Conn{}); err == nil {
		t.Error("want error when ReadFile is nil")
	}
}

func TestCloudflaredListRulesConfigNotFound(t *testing.T) {
	cf := &Cloudflared{}
	conn := Conn{
		ReadFile: func(_ context.Context, _ string) (io.ReadCloser, error) {
			return nil, io.ErrUnexpectedEOF // every path fails
		},
	}
	if _, err := cf.ListRules(context.Background(), conn); err == nil {
		t.Error("want error when config file is not found on any candidate path")
	}
}

func TestCloudflaredMutationsUnsupported(t *testing.T) {
	cf := &Cloudflared{}
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

func TestMountBasedCandidates(t *testing.T) {
	mounts := []db.MountRow{
		{Source: "/home/user/cloudflared", Destination: "/etc/cloudflared", Type: "bind"},
		{Source: "/data/nginx.conf", Destination: "/etc/nginx/nginx.conf", Type: "bind"},
	}
	got := mountBasedCandidates(mounts)
	want := []string{
		"/home/user/cloudflared/config.yml",
		"/home/user/cloudflared/config.yaml",
	}
	if len(got) != len(want) {
		t.Fatalf("mountBasedCandidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMountBasedCandidatesDirectFile(t *testing.T) {
	mounts := []db.MountRow{
		{Source: "/srv/cloudflared/config.yml", Destination: "/etc/cloudflared/config.yml", Type: "bind"},
	}
	got := mountBasedCandidates(mounts)
	if len(got) != 1 || got[0] != "/srv/cloudflared/config.yml" {
		t.Errorf("mountBasedCandidates = %v, want [/srv/cloudflared/config.yml]", got)
	}
}

func TestCloudflaredDetection(t *testing.T) {
	cases := []struct {
		image string
	}{
		{"cloudflare/cloudflared:latest"},
		{"cloudflared:2024.1.0"},
		{"ghcr.io/cloudflare/cloudflared:latest"},
	}
	for _, tc := range cases {
		a := DetectByImages([]string{tc.image})
		if a == nil || a.Name() != "cloudflared" {
			t.Errorf("image %q: got %v, want cloudflared", tc.image, a)
		}
	}
}

func TestCloudflaredUseMountCandidatesFirst(t *testing.T) {
	// Verify that mount candidates are tried before default paths.
	tried := []string{}
	conn := Conn{
		MountCandidates: []string{"/host/cloudflared/config.yml"},
		ReadFile: func(_ context.Context, p string) (io.ReadCloser, error) {
			tried = append(tried, p)
			if p == "/host/cloudflared/config.yml" {
				return io.NopCloser(strings.NewReader(sampleConfig)), nil
			}
			return nil, io.ErrUnexpectedEOF
		},
	}
	cf := &Cloudflared{}
	rules, err := cf.ListRules(context.Background(), conn)
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}
	// The mount candidate must have been tried first.
	if len(tried) == 0 || tried[0] != "/host/cloudflared/config.yml" {
		t.Errorf("expected mount candidate tried first, tried = %v", tried)
	}
}
