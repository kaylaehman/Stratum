package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password stored in plaintext")
	}
	if err := CheckPassword(hash, "correct horse battery staple"); err != nil {
		t.Errorf("CheckPassword correct: %v", err)
	}
	if err := CheckPassword(hash, "wrong"); err == nil {
		t.Error("CheckPassword accepted wrong password")
	}
}

func TestJWTIssueVerify(t *testing.T) {
	j := NewJWT([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	token, jti, exp, err := j.Issue("user-123")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if jti == "" || exp.Before(time.Now()) {
		t.Errorf("bad jti/exp: %q %v", jti, exp)
	}
	sub, err := j.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if sub != "user-123" {
		t.Errorf("sub = %q, want user-123", sub)
	}
}

func TestJWTRejectsExpired(t *testing.T) {
	j := NewJWT([]byte("0123456789abcdef0123456789abcdef"), -time.Minute)
	token, _, _, _ := j.Issue("u")
	if _, err := j.Verify(token); err == nil {
		t.Error("Verify accepted an expired token")
	}
}

func TestJWTRejectsWrongSecret(t *testing.T) {
	a := NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Minute)
	b := NewJWT([]byte("ffffffffffffffffffffffffffffffff"), time.Minute)
	token, _, _, _ := a.Issue("u")
	if _, err := b.Verify(token); err == nil {
		t.Error("Verify accepted a token signed with a different secret")
	}
}

func TestRefreshTokenGenerationAndHash(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if raw == "" || hash == "" || raw == hash {
		t.Fatalf("bad token/hash: %q %q", raw, hash)
	}
	if strings.ContainsAny(raw, "+/=") {
		t.Errorf("raw token not base64url: %q", raw)
	}
	if HashRefreshToken(raw) != hash {
		t.Error("HashRefreshToken not deterministic")
	}
	raw2, _, _ := GenerateRefreshToken()
	if raw2 == raw {
		t.Error("two refresh tokens were identical")
	}
}
