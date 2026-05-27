// Package totp implements RFC 6238 time-based one-time passwords (Feature 7),
// in-house (no third-party dependency). Secrets are base32 (authenticator-app
// compatible); codes are 6 digits over SHA-1 with a 30s step. Verify allows ±1
// step for clock skew. Unit-tested against the RFC 6238 test vectors.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	step    = 30 * time.Second
	digits  = 6
	skewWin = 1 // accept the previous/next step too
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret returns a new random base32 secret (20 bytes → 32 chars).
func GenerateSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return b32.EncodeToString(buf), nil
}

// Code computes the 6-digit TOTP for a secret at time t.
func Code(secret string, t time.Time) (string, error) {
	key, err := b32.DecodeString(normalizeSecret(secret))
	if err != nil {
		return "", fmt.Errorf("totp: bad secret: %w", err)
	}
	return codeForCounter(key, uint64(t.Unix())/uint64(step.Seconds())), nil
}

func codeForCounter(key []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	trunc := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])
	mod := trunc % 1_000_000
	return fmt.Sprintf("%06d", mod)
}

// Verify checks code against the secret within ±1 step of t (clock-skew
// tolerant), using a constant-time comparison.
func Verify(secret, code string, t time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != digits {
		return false
	}
	key, err := b32.DecodeString(normalizeSecret(secret))
	if err != nil {
		return false
	}
	base := uint64(t.Unix()) / uint64(step.Seconds())
	for d := -skewWin; d <= skewWin; d++ {
		candidate := codeForCounter(key, base+uint64(int64(d)))
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// ProvisioningURI builds an otpauth:// URI for authenticator-app enrollment.
func ProvisioningURI(secret, account, issuer string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", normalizeSecret(secret))
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	return "otpauth://totp/" + label + "?" + q.Encode()
}

func normalizeSecret(s string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
}
