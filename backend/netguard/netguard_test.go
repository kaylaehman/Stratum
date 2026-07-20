package netguard

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestBlocked(t *testing.T) {
	block := []string{
		"169.254.169.254", // cloud metadata
		"10.0.0.5",
		"172.16.4.4",
		"192.168.1.10",
		"0.0.0.0",
		"fe80::1",
		"fd00::1", // unique-local
	}
	for _, s := range block {
		if !blocked(net.ParseIP(s)) {
			t.Errorf("blocked(%s) = false, want true", s)
		}
	}
	allow := []string{
		"127.0.0.1", // loopback — local Ollama
		"::1",
		"8.8.8.8", // public
		"1.1.1.1",
	}
	for _, s := range allow {
		if blocked(net.ParseIP(s)) {
			t.Errorf("blocked(%s) = true, want false", s)
		}
	}
}

// dialErr calls the guarded DialContext against a literal IP (no DNS needed) and
// returns the resulting error string.
func dialErr(t *testing.T, allow []string, addr string) string {
	t.Helper()
	tr := Transport(allow)
	_, err := tr.DialContext(context.Background(), "tcp", addr)
	if err == nil {
		t.Fatalf("dial %s: expected an error (nothing is listening), got nil", addr)
	}
	return err.Error()
}

func TestTransport_RefusesInternal(t *testing.T) {
	// A blocked address is refused up front, never dialed.
	if e := dialErr(t, nil, "169.254.169.254:80"); !strings.Contains(e, "refusing") {
		t.Errorf("metadata endpoint: err = %q, want a refusal", e)
	}
	if e := dialErr(t, nil, "192.168.1.50:8080"); !strings.Contains(e, "refusing") {
		t.Errorf("LAN address: err = %q, want a refusal", e)
	}
}

func TestTransport_AllowsLoopbackAndAllowlist(t *testing.T) {
	// Loopback is permitted, so the failure is a dial/connect error, not a refusal.
	if e := dialErr(t, nil, "127.0.0.1:1"); strings.Contains(e, "refusing") {
		t.Errorf("loopback: err = %q, must not be refused", e)
	}
	// An explicitly allowlisted LAN host (a remote Ollama) bypasses the IP block.
	if e := dialErr(t, []string{"192.168.1.50"}, "192.168.1.50:1"); strings.Contains(e, "refusing") {
		t.Errorf("allowlisted host: err = %q, must not be refused", e)
	}
}
