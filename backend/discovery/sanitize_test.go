package discovery_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kaylaehman/stratum/backend/discovery"
)

func TestSanitizeStripsHostAndCredentialContext(t *testing.T) {
	// A realistic x/crypto/ssh error carrying host, port, and username.
	raw := errors.New(`ssh: handshake failed for host 192.168.1.10:22 user "root": unable to authenticate, attempted methods [none publickey]`)
	got := discovery.SanitizeProbeError(raw)

	if got != discovery.ErrCategorySSHAuthFailed {
		t.Errorf("category = %q, want %q", got, discovery.ErrCategorySSHAuthFailed)
	}
	// The category must not leak any host/port/user detail.
	for _, leak := range []string{"192.168.1.10", "22", "root"} {
		if strings.Contains(got, leak) {
			t.Errorf("sanitized output %q leaks %q", got, leak)
		}
	}
}

func TestSanitizeCategories(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		{"knownhosts: key mismatch for host 10.0.0.5", discovery.ErrCategorySSHHostKey},
		{"ssh: handshake failed: permission denied", discovery.ErrCategorySSHAuthFailed},
		{"x509: certificate signed by unknown authority", discovery.ErrCategoryTLS},
		{"Get https://10.0.0.5:8006/api2/json/version: 401 Unauthorized", discovery.ErrCategoryProxmoxUnauthed},
		{"dial tcp 10.0.0.5:22: connect: connection refused", discovery.ErrCategorySSHUnreachable},
		{"context deadline exceeded (i/o timeout)", discovery.ErrCategoryTimeout},
		{"cannot connect to the docker daemon at unix:///var/run/docker.sock", discovery.ErrCategoryDockerUnreachable},
		{"something totally unexpected happened", discovery.ErrCategoryUnknown},
	}
	for _, c := range cases {
		if got := discovery.SanitizeProbeError(errors.New(c.msg)); got != c.want {
			t.Errorf("Sanitize(%q) = %q, want %q", c.msg, got, c.want)
		}
	}
}

func TestSanitizeNil(t *testing.T) {
	if got := discovery.SanitizeProbeError(nil); got != "" {
		t.Errorf("Sanitize(nil) = %q, want empty", got)
	}
}
