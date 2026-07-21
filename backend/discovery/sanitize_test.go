package discovery_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/KAE-Labs/stratum/backend/discovery"
)

func TestSanitizeStripsHostAndCredentialContext(t *testing.T) {
	// A realistic x/crypto/ssh error carrying host, port, and username.
	raw := errors.New(`ssh: handshake failed for host 192.168.1.10:22 user "root": unable to authenticate, attempted methods [none publickey]`)
	cat, hint := discovery.SanitizeProbeError(raw)

	if cat != discovery.ErrCategorySSHAuthPubkeyRejected {
		t.Errorf("category = %q, want %q", cat, discovery.ErrCategorySSHAuthPubkeyRejected)
	}
	// Neither the category nor the hint may leak host/port/user detail.
	for _, leak := range []string{"192.168.1.10", "22", "root"} {
		if strings.Contains(cat, leak) {
			t.Errorf("sanitized category %q leaks %q", cat, leak)
		}
		if strings.Contains(hint, leak) {
			t.Errorf("hint %q leaks %q", hint, leak)
		}
	}
	if hint == "" {
		t.Errorf("expected non-empty hint for %q", cat)
	}
}

func TestSanitizeCategories(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want string
	}{
		// SSH auth subcategorization — the analyzer relies on the
		// "attempted methods [...]" tail x/crypto/ssh adds to its
		// handshake error to tell pubkey-rejected apart from
		// password-rejected.
		{"pubkey rejected", "ssh: handshake failed: unable to authenticate, attempted methods [none publickey]", discovery.ErrCategorySSHAuthPubkeyRejected},
		{"password rejected", "ssh: handshake failed: unable to authenticate, attempted methods [none password]", discovery.ErrCategorySSHAuthPasswordWrong},
		{"generic auth fail (no method tail)", "ssh: handshake failed: permission denied", discovery.ErrCategorySSHAuthFailed},

		// Private-key parsing failures — surface BEFORE the network
		// handshake. x/crypto/ssh returns "ssh: this private key is
		// passphrase protected" when ParsePrivateKey hits an encrypted
		// key with no passphrase; "x509: decryption password incorrect"
		// when the passphrase is wrong.
		{"passphrase missing", "ssh: this private key is passphrase protected", discovery.ErrCategorySSHPassphraseRequired},
		{"passphrase wrong", "x509: decryption password incorrect", discovery.ErrCategorySSHPassphraseWrong},

		// Self-target and post-handshake detection failure must NOT be
		// swallowed by the generic ssh:/dial cases and mislabeled unreachable.
		{"target is self", "ssh: probe target resolves to the stratum host itself", discovery.ErrCategorySSHTargetSelf},
		{"detect failed after handshake", "ssh: detection session failed: session run: Process exited with status 1", discovery.ErrCategorySSHDetectFailed},

		{"host key mismatch", "knownhosts: key mismatch for host 10.0.0.5", discovery.ErrCategorySSHHostKey},
		{"tls error", "x509: certificate signed by unknown authority", discovery.ErrCategoryTLS},
		{"proxmox unauthed", "Get https://10.0.0.5:8006/api2/json/version: 401 Unauthorized", discovery.ErrCategoryProxmoxUnauthed},
		{"unreachable", "dial tcp 10.0.0.5:22: connect: connection refused", discovery.ErrCategorySSHUnreachable},
		{"timeout", "context deadline exceeded (i/o timeout)", discovery.ErrCategoryTimeout},
		{"docker unreachable", "cannot connect to the docker daemon at unix:///var/run/docker.sock", discovery.ErrCategoryDockerUnreachable},

		// Docker TLS config build failures surface before any network I/O. They
		// must categorize as a TLS/cert problem, NOT docker_unreachable — even
		// though the error text begins with "docker:".
		{"docker bad CA PEM", "docker: build TLS config: docker: failed to parse CA PEM", discovery.ErrCategoryTLS},
		{"docker cert without key", "docker: build TLS config: docker: client cert and key must be supplied together", discovery.ErrCategoryTLS},
		{"docker client keypair parse", "docker: build TLS config: docker: parse client cert/key: tls: failed to find any PEM data in key input", discovery.ErrCategoryTLS},

		{"unknown", "something totally unexpected happened", discovery.ErrCategoryUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cat, hint := discovery.SanitizeProbeError(errors.New(c.msg))
			if cat != c.want {
				t.Errorf("category = %q, want %q (msg=%q)", cat, c.want, c.msg)
			}
			if hint == "" {
				t.Errorf("hint is empty for category %q", cat)
			}
		})
	}
}

func TestSanitizeNil(t *testing.T) {
	cat, hint := discovery.SanitizeProbeError(nil)
	if cat != "" || hint != "" {
		t.Errorf("Sanitize(nil) = (%q, %q), want empty", cat, hint)
	}
}
