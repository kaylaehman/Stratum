package sshkeys

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// genKey builds a real, parseable authorized_keys line plus its fingerprint.
func genKey(t *testing.T, comment string) (line, fingerprint string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	line = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))) + " " + comment
	return line, ssh.FingerprintSHA256(sshPub)
}

func TestParseAuditSkipsCommentsAndJunk(t *testing.T) {
	keyA, fpA := genKey(t, "kayla@laptop")
	keyB, fpB := genKey(t, "ci@runner")
	out := strings.Join([]string{
		"root\t/root/.ssh/authorized_keys\t" + keyA,
		"root\t/root/.ssh/authorized_keys\t# a comment line",
		"kayla\t/home/kayla/.ssh/authorized_keys\t" + keyB,
		"kayla\t/home/kayla/.ssh/authorized_keys\tnot-a-valid-key-line",
		"malformed-no-tabs",
	}, "\n")
	entries := parseAudit(out)
	if len(entries) != 2 {
		t.Fatalf("parsed %d entries, want 2: %+v", len(entries), entries)
	}
	if entries[0].User != "root" || entries[0].Comment != "kayla@laptop" || entries[0].Type != "ssh-ed25519" {
		t.Errorf("entry0 = %+v", entries[0])
	}
	if entries[0].Fingerprint != fpA || entries[1].Fingerprint != fpB {
		t.Errorf("fingerprints = %q,%q want %q,%q", entries[0].Fingerprint, entries[1].Fingerprint, fpA, fpB)
	}
	if !strings.HasPrefix(entries[0].Fingerprint, "SHA256:") {
		t.Errorf("fingerprint = %q, want SHA256: prefix", entries[0].Fingerprint)
	}
}

func TestFilterKeyRemovesByFingerprint(t *testing.T) {
	keyA, fpA := genKey(t, "kayla@laptop")
	keyB, _ := genKey(t, "ci@runner")

	content := "# header comment\n" + keyA + "\n\n" + keyB + "\n"
	got, removed := filterKey(content, fpA)
	if !removed {
		t.Fatal("expected a line removed")
	}
	if strings.Contains(got, "kayla@laptop") {
		t.Error("keyA should have been removed")
	}
	if !strings.Contains(got, "ci@runner") {
		t.Error("keyB should be preserved")
	}
	if !strings.Contains(got, "# header comment") {
		t.Error("comment line should be preserved")
	}
	if _, removed := filterKey(content, "SHA256:doesnotexist"); removed {
		t.Error("unknown fingerprint should not remove anything")
	}
}

func TestValidKeyPath(t *testing.T) {
	ok := []string{"/root/.ssh/authorized_keys", "/home/kayla/.ssh/authorized_keys"}
	for _, p := range ok {
		if !ValidKeyPath(p) {
			t.Errorf("%q should be valid", p)
		}
	}
	bad := []string{"/etc/passwd", "relative/.ssh/authorized_keys", "/home/x/.ssh/authorized_keys/../../../etc/passwd", "/root/.ssh/known_hosts"}
	for _, p := range bad {
		if ValidKeyPath(p) {
			t.Errorf("%q should be rejected", p)
		}
	}
}
