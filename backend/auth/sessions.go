package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// refreshTokenBytes is the entropy of a refresh token before encoding.
const refreshTokenBytes = 32

// GenerateRefreshToken returns a base64url raw token (given to the client) and
// its sha256 hex hash (stored server-side; the raw token is never persisted).
func GenerateRefreshToken() (raw, hash string, err error) {
	b := make([]byte, refreshTokenBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("auth: read random: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashRefreshToken(raw), nil
}

// HashRefreshToken returns the sha256 hex hash of a raw refresh token. Use a
// constant-time compare against the stored hash when validating.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
