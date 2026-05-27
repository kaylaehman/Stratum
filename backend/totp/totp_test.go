package totp

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// rfcSecret is the RFC 6238 test seed "12345678901234567890" in base32.
func rfcSecret(t *testing.T) string {
	t.Helper()
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
}

// TestRFC6238Vectors checks the 6-digit truncations of the published SHA-1
// vectors (the canonical 8-digit values' last 6 digits).
func TestRFC6238Vectors(t *testing.T) {
	secret := rfcSecret(t)
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		got, err := Code(secret, time.Unix(c.unix, 0))
		if err != nil {
			t.Fatalf("Code(%d): %v", c.unix, err)
		}
		if got != c.want {
			t.Errorf("Code at t=%d = %q, want %q", c.unix, got, c.want)
		}
	}
}

func TestVerifySkewAndReject(t *testing.T) {
	secret := rfcSecret(t)
	now := time.Unix(1111111109, 0)
	code, _ := Code(secret, now)
	if !Verify(secret, code, now) {
		t.Error("current code should verify")
	}
	// One step earlier/later should still verify (±1 skew window).
	if !Verify(secret, code, now.Add(25*time.Second)) {
		t.Error("code should verify within the next step (skew)")
	}
	// Two steps away must NOT verify.
	if Verify(secret, code, now.Add(90*time.Second)) {
		t.Error("code two steps away should be rejected")
	}
	// Wrong code rejected; non-6-digit rejected.
	if Verify(secret, "000000", now) && code != "000000" {
		t.Error("wrong code should be rejected")
	}
	if Verify(secret, "12345", now) {
		t.Error("non-6-digit code should be rejected")
	}
}

func TestSecretAndURI(t *testing.T) {
	s, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 {
		t.Errorf("secret len = %d, want 32 base32 chars", len(s))
	}
	uri := ProvisioningURI(s, "kayla", "Stratum")
	if !strings.HasPrefix(uri, "otpauth://totp/Stratum:kayla?") || !strings.Contains(uri, "secret="+s) {
		t.Errorf("uri = %q", uri)
	}
	// A freshly generated secret must produce a verifiable code.
	code, _ := Code(s, time.Now())
	if !Verify(s, code, time.Now()) {
		t.Error("generated secret should round-trip code/verify")
	}
}
