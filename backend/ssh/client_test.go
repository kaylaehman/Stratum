package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ── parseDetection tests ──────────────────────────────────────────────────────

func TestParseDetection_FullOutput(t *testing.T) {
	raw := `NAME="Ubuntu"
VERSION="22.04"
---STRATUM---
systemctl
run_systemd
crontab
docker
etc_pve
`
	d := parseDetection(raw)
	if d.OSReleaseRaw != "NAME=\"Ubuntu\"\nVERSION=\"22.04\"" {
		t.Errorf("OSReleaseRaw = %q, want ubuntu os-release body", d.OSReleaseRaw)
	}
	if !d.HasSystemctl {
		t.Error("HasSystemctl should be true")
	}
	if !d.HasRunSystemd {
		t.Error("HasRunSystemd should be true")
	}
	if !d.HasCrontab {
		t.Error("HasCrontab should be true")
	}
	if !d.HasDocker {
		t.Error("HasDocker should be true")
	}
	if !d.HasEtcPve {
		t.Error("HasEtcPve should be true")
	}
}

func TestParseDetection_EmptyOSRelease(t *testing.T) {
	// /etc/os-release missing → cat exits non-zero but we still get the marker.
	raw := `---STRATUM---
systemctl
crontab
`
	d := parseDetection(raw)
	if d.OSReleaseRaw != "" {
		t.Errorf("OSReleaseRaw = %q, want empty string", d.OSReleaseRaw)
	}
	if !d.HasSystemctl {
		t.Error("HasSystemctl should be true")
	}
	if !d.HasCrontab {
		t.Error("HasCrontab should be true")
	}
	if d.HasDocker {
		t.Error("HasDocker should be false")
	}
	if d.HasEtcPve {
		t.Error("HasEtcPve should be false")
	}
	if d.HasRunSystemd {
		t.Error("HasRunSystemd should be false")
	}
}

func TestParseDetection_AlpineNoCrontabNoSystemctl(t *testing.T) {
	// Alpine-style: BusyBox, no systemctl, but crontab present via busybox.
	raw := `NAME="Alpine Linux"
ID=alpine
---STRATUM---
crontab
docker
`
	d := parseDetection(raw)
	if d.OSReleaseRaw == "" {
		t.Error("OSReleaseRaw should not be empty for Alpine")
	}
	if d.HasSystemctl {
		t.Error("HasSystemctl should be false on Alpine")
	}
	if d.HasRunSystemd {
		t.Error("HasRunSystemd should be false on Alpine")
	}
	if !d.HasCrontab {
		t.Error("HasCrontab should be true on Alpine")
	}
	if !d.HasDocker {
		t.Error("HasDocker should be true")
	}
}

func TestParseDetection_ProxmoxNode(t *testing.T) {
	raw := `PRETTY_NAME="Debian GNU/Linux 12"
---STRATUM---
systemctl
run_systemd
crontab
etc_pve
`
	d := parseDetection(raw)
	if !d.HasEtcPve {
		t.Error("HasEtcPve should be true on Proxmox node")
	}
	if !d.HasSystemctl {
		t.Error("HasSystemctl should be true on Proxmox/Debian node")
	}
	if d.HasDocker {
		t.Error("HasDocker should be false — Proxmox node without Docker")
	}
}

func TestParseDetection_NoMarker(t *testing.T) {
	// Fallback: no marker at all — treat all as os-release.
	raw := "some unexpected output"
	d := parseDetection(raw)
	if d.OSReleaseRaw != "some unexpected output" {
		t.Errorf("OSReleaseRaw = %q, want full raw output", d.OSReleaseRaw)
	}
	if d.HasSystemctl || d.HasCrontab || d.HasDocker || d.HasEtcPve || d.HasRunSystemd {
		t.Error("all bool fields should be false when marker is absent")
	}
}

func TestParseDetection_OnlyMarkerNoTokens(t *testing.T) {
	raw := `NAME="Debian"
---STRATUM---
`
	d := parseDetection(raw)
	if d.OSReleaseRaw != `NAME="Debian"` {
		t.Errorf("OSReleaseRaw = %q", d.OSReleaseRaw)
	}
	if d.HasSystemctl || d.HasCrontab || d.HasDocker || d.HasEtcPve || d.HasRunSystemd {
		t.Error("all bool fields should be false")
	}
}

// ── host-key callback tests ───────────────────────────────────────────────────

// genEd25519 generates a fresh ed25519 key pair and returns the ssh.PublicKey and Signer.
func genEd25519(t *testing.T) (ssh.PublicKey, ssh.Signer) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh.NewPublicKey: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("ssh.NewSignerFromKey: %v", err)
	}
	return sshPub, signer
}

// makeKnownHostsLine builds a known_hosts line for host:port and key.
func makeKnownHostsLine(host string, port int, key ssh.PublicKey) string {
	normalized := knownhosts.Normalize(net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	return knownhosts.Line([]string{normalized}, key)
}

// fakeAddr implements net.Addr for callback invocation.
type fakeAddr struct{ s string }

func (f fakeAddr) Network() string { return "tcp" }
func (f fakeAddr) String() string  { return f.s }

func TestTOFUCallback_RecordsKeyAndAccepts(t *testing.T) {
	pub, _ := genEd25519(t)
	var captured HostKey
	cb := toFUCallback("192.0.2.1", 22, &captured)

	addr := fakeAddr{"192.0.2.1:22"}
	if err := cb("192.0.2.1:22", addr, pub); err != nil {
		t.Fatalf("TOFU callback returned error: %v", err)
	}

	if captured.SHA256 == "" {
		t.Error("SHA256 should be populated after TOFU callback")
	}
	if captured.KnownHostsLine == "" {
		t.Error("KnownHostsLine should be populated after TOFU callback")
	}
	// SHA256 should match what ssh.FingerprintSHA256 would return.
	want := ssh.FingerprintSHA256(pub)
	if captured.SHA256 != want {
		t.Errorf("SHA256 = %q, want %q", captured.SHA256, want)
	}
}

func TestPinnedCallback_SameKeyPasses(t *testing.T) {
	pub, _ := genEd25519(t)
	line := makeKnownHostsLine("192.0.2.1", 22, pub)

	var mismatch bool
	cb, err := pinnedCallback(line, &mismatch)
	if err != nil {
		t.Fatalf("pinnedCallback: %v", err)
	}

	addr := fakeAddr{"192.0.2.1:22"}
	if err := cb("192.0.2.1:22", addr, pub); err != nil {
		t.Errorf("same key should pass, got error: %v", err)
	}
	if mismatch {
		t.Error("mismatch flag should be false for matching key")
	}
}

func TestPinnedCallback_DifferentKeyFails(t *testing.T) {
	pub1, _ := genEd25519(t)
	pub2, _ := genEd25519(t)

	// Pin key1, then present key2.
	line := makeKnownHostsLine("192.0.2.1", 22, pub1)

	var mismatch bool
	cb, err := pinnedCallback(line, &mismatch)
	if err != nil {
		t.Fatalf("pinnedCallback: %v", err)
	}

	addr := fakeAddr{"192.0.2.1:22"}
	err = cb("192.0.2.1:22", addr, pub2)
	if err == nil {
		t.Fatal("expected error for mismatched host key, got nil")
	}
	if !errors.Is(err, ErrHostKeyMismatch) {
		t.Errorf("error should wrap ErrHostKeyMismatch, got: %v", err)
	}
	if !mismatch {
		t.Error("mismatch flag should be set on a mismatched key")
	}
	if !containsSubstring(err.Error(), "host key mismatch") {
		t.Errorf("error text should contain 'host key mismatch' (sanitizer compat), got: %v", err)
	}
}

func TestBuildHostKeyCallback_TOFUWhenNoPinned(t *testing.T) {
	var captured HostKey
	var mismatch bool
	cb, err := buildHostKeyCallback("10.0.0.1", 22, "", &captured, &mismatch)
	if err != nil {
		t.Fatalf("buildHostKeyCallback: %v", err)
	}
	pub, _ := genEd25519(t)
	if err := cb("10.0.0.1:22", fakeAddr{"10.0.0.1:22"}, pub); err != nil {
		t.Errorf("TOFU should accept any key: %v", err)
	}
	if captured.KnownHostsLine == "" {
		t.Error("KnownHostsLine should be set after TOFU")
	}
}

func TestBuildHostKeyCallback_PinnedLineVerifies(t *testing.T) {
	pub, _ := genEd25519(t)
	line := makeKnownHostsLine("10.0.0.1", 22, pub)

	var captured HostKey
	var mismatch bool
	cb, err := buildHostKeyCallback("10.0.0.1", 22, line, &captured, &mismatch)
	if err != nil {
		t.Fatalf("buildHostKeyCallback: %v", err)
	}

	// Correct key passes.
	if err := cb("10.0.0.1:22", fakeAddr{"10.0.0.1:22"}, pub); err != nil {
		t.Errorf("matching key should pass: %v", err)
	}

	// Different key fails.
	pub2, _ := genEd25519(t)
	err = cb("10.0.0.1:22", fakeAddr{"10.0.0.1:22"}, pub2)
	if err == nil || !errors.Is(err, ErrHostKeyMismatch) {
		t.Errorf("mismatched key should wrap ErrHostKeyMismatch, got: %v", err)
	}
}

// ── detection-command exit handling ───────────────────────────────────────────

func TestIsFatalRunErr(t *testing.T) {
	// A non-zero command exit is NOT fatal: the detection command's final
	// `test -d /etc/pve` exits 1 on every non-Proxmox host, and stdout is still
	// the real signal. Treating it as fatal is the Bug-2 regression.
	if isFatalRunErr(&ssh.ExitError{}) {
		t.Error("a non-zero command exit (*ssh.ExitError) must NOT be fatal")
	}
	// nil → nothing went wrong.
	if isFatalRunErr(nil) {
		t.Error("nil error must not be fatal")
	}
	// A genuine session/transport failure IS fatal.
	if !isFatalRunErr(errors.New("new session: ssh: disconnect")) {
		t.Error("a non-exit error must be fatal")
	}
}

// containsSubstring is a helper so tests don't import strings directly.
func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
