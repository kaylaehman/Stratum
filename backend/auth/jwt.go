package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWT issues and verifies HS256 access tokens.
type JWT struct {
	secret []byte
	ttl    time.Duration
}

// NewJWT builds a JWT issuer with the given HMAC secret and access-token TTL.
func NewJWT(secret []byte, ttl time.Duration) *JWT {
	return &JWT{secret: secret, ttl: ttl}
}

// Issue returns a signed access token plus its jti and expiry. Claims:
// sub (user id), iat, exp, jti.
func (j *JWT) Issue(userID string) (token, jti string, exp time.Time, err error) {
	jti = uuid.NewString()
	now := time.Now()
	exp = now.Add(j.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ID:        jti,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, jti, exp, nil
}

// Verify parses and validates a token, returning the subject (user id). It
// rejects expired tokens and any non-HS256 algorithm.
func (j *JWT) Verify(tokenStr string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", t.Header["alg"])
		}
		return j.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return "", fmt.Errorf("auth: verify token: %w", err)
	}
	return claims.Subject, nil
}
