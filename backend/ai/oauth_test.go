package ai

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGeneratePKCE(t *testing.T) {
	p, err := GeneratePKCE()
	if err != nil {
		t.Fatal(err)
	}
	if p.Verifier == "" || p.Challenge == "" {
		t.Fatal("empty verifier/challenge")
	}
	sum := sha256.Sum256([]byte(p.Verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); p.Challenge != want {
		t.Errorf("challenge = %q, want S256(verifier) = %q", p.Challenge, want)
	}
	p2, _ := GeneratePKCE()
	if p2.Verifier == p.Verifier {
		t.Error("two PKCE verifiers should differ")
	}
}

func TestAuthorizeURL(t *testing.T) {
	u := AuthorizeURL("chal123", "state456")
	if !strings.HasPrefix(u, oauthAuthorizeURL) {
		t.Errorf("wrong base: %s", u)
	}
	for _, want := range []string{
		"code_challenge=chal123", "code_challenge_method=S256",
		"state=state456", "response_type=code", "client_id=",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("authorize URL missing %q: %s", want, u)
		}
	}
}

func TestSplitPastedCode(t *testing.T) {
	if c, s := SplitPastedCode("  abc#xyz "); c != "abc" || s != "xyz" {
		t.Errorf("got (%q,%q), want (abc,xyz)", c, s)
	}
	if c, s := SplitPastedCode("plain"); c != "plain" || s != "" {
		t.Errorf("got (%q,%q), want (plain,'')", c, s)
	}
}

func TestTokenExpired(t *testing.T) {
	now := time.Unix(1000, 0)
	if tokenExpired(time.Time{}, now) {
		t.Error("zero expiry should be treated as non-expiring")
	}
	if !tokenExpired(now.Add(30*time.Second), now) {
		t.Error("expiry within the refresh skew should be considered expired")
	}
	if tokenExpired(now.Add(10*time.Minute), now) {
		t.Error("far-future expiry should be valid")
	}
}

func TestExchangeAndRefresh(t *testing.T) {
	var gotGrant string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		gotGrant = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","refresh_token":"RT","expires_in":3600}`))
	}))
	defer srv.Close()
	old := tokenURL
	tokenURL = srv.URL
	defer func() { tokenURL = old }()

	now := time.Unix(2000, 0)
	ts, err := ExchangeCode(context.Background(), srv.Client(), "thecode", "theverifier", "st", now)
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if ts.AccessToken != "AT" || ts.RefreshToken != "RT" {
		t.Errorf("tokens = %+v", ts)
	}
	if !ts.ExpiresAt.Equal(now.Add(3600 * time.Second)) {
		t.Errorf("ExpiresAt = %v, want now+3600s", ts.ExpiresAt)
	}
	if !strings.Contains(gotGrant, "authorization_code") || !strings.Contains(gotGrant, "theverifier") {
		t.Errorf("exchange payload missing grant/verifier: %s", gotGrant)
	}

	if ts2, err := RefreshToken(context.Background(), srv.Client(), "RT", now); err != nil || ts2.AccessToken != "AT" {
		t.Errorf("RefreshToken = (%+v, %v)", ts2, err)
	}
	if !strings.Contains(gotGrant, "refresh_token") {
		t.Errorf("refresh payload missing grant: %s", gotGrant)
	}
}

func TestExchangeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"bad code"}`))
	}))
	defer srv.Close()
	old := tokenURL
	tokenURL = srv.URL
	defer func() { tokenURL = old }()

	_, err := ExchangeCode(context.Background(), srv.Client(), "x", "y", "z", time.Now())
	if err == nil || !strings.Contains(err.Error(), "bad code") {
		t.Errorf("want error containing description, got %v", err)
	}
}
