package fs

import "testing"

func TestIsSensitiveRead(t *testing.T) {
	// Credential files that must never be served through the browser.
	blocked := []string{
		"/etc/shadow",
		"/etc/gshadow",
		"/etc/shadow-",
		"/root/.ssh/id_rsa",
		"/home/kayla/.ssh/id_ed25519",
		"/home/u/.ssh/id_prod", // custom-named private key under .ssh
		"/home/u/.ssh/deploy.pem",
		"/etc/ssl/private/server.key",
		"/etc/ssl/private",
	}
	for _, p := range blocked {
		if !IsSensitiveRead(p) {
			t.Errorf("IsSensitiveRead(%q) = false, want true (credential file must be blocked)", p)
		}
	}

	// Legitimate config/inspection reads that must stay allowed — the whole point
	// of the file browser. A too-broad deny-list would regress this.
	allowed := []string{
		"/etc/hosts",
		"/etc/fstab",
		"/etc/docker/daemon.json",
		"/root/.ssh/id_rsa.pub",       // public key
		"/root/.ssh/authorized_keys",  // not a private key
		"/root/.ssh/known_hosts",
		"/var/log/syslog",
		"/home/u/app/config.yaml",
	}
	for _, p := range allowed {
		if IsSensitiveRead(p) {
			t.Errorf("IsSensitiveRead(%q) = true, want false (legit read must not be blocked)", p)
		}
	}
}
