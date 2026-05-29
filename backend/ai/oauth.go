package ai

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Anthropic's public Claude Code OAuth client parameters (the "claude.ai -p"
// method, Feature 31). These are the same values the Claude Code CLI uses, so
// an operator can authenticate Stratum's assistant with their Claude
// subscription instead of pasting a raw API key.
//
// NOTE: these are unofficial/undocumented endpoints; the live handshake cannot
// be exercised in CI. The PKCE generation, URL building, and token-response
// parsing below are unit-tested; the network round-trip is best-effort.
const (
	oauthAuthorizeURL = "https://claude.ai/oauth/authorize"
	oauthClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	oauthRedirectURI  = "https://console.anthropic.com/oauth/code/callback"
	oauthScopes       = "org:create_api_key user:profile user:inference"
	// oauthBetaHeader is required when authenticating the Messages API with an
	// OAuth bearer token instead of an x-api-key.
	oauthBetaHeader = "oauth-2025-04-20"
	// refreshSkew refreshes a token slightly before it actually expires so an
	// in-flight request doesn't race the expiry boundary.
	refreshSkew = 60 * time.Second
)

// tokenURL is a var (not const) so tests can point it at a stub server.
var tokenURL = "https://console.anthropic.com/v1/oauth/token"

// TokenSet is the result of an OAuth code exchange or refresh.
type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// PKCE is a generated verifier/challenge pair.
type PKCE struct {
	Verifier  string
	Challenge string
}

// GeneratePKCE returns a fresh PKCE verifier and its S256 challenge.
func GeneratePKCE() (PKCE, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return PKCE{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(sum[:])}, nil
}

// GenerateState returns a random opaque CSRF state value.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthorizeURL builds the Claude OAuth authorize URL for a PKCE challenge.
// state is an opaque value echoed back to detect a mismatched flow.
func AuthorizeURL(challenge, state string) string {
	q := url.Values{}
	q.Set("code", "true")
	q.Set("client_id", oauthClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", oauthRedirectURI)
	q.Set("scope", oauthScopes)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	return oauthAuthorizeURL + "?" + q.Encode()
}

// SplitPastedCode handles the "code#state" value Claude's callback page shows:
// it returns (code, state). A plain code with no "#" returns (code, "").
func SplitPastedCode(pasted string) (code, state string) {
	pasted = strings.TrimSpace(pasted)
	if i := strings.Index(pasted, "#"); i >= 0 {
		return pasted[:i], pasted[i+1:]
	}
	return pasted, ""
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// postToken posts a JSON token request and parses the result. now is injected so
// expiry math is deterministic in tests.
func postToken(ctx context.Context, hc *http.Client, payload map[string]string, now time.Time) (TokenSet, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return TokenSet{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return TokenSet{}, fmt.Errorf("oauth: token request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var tr oauthTokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return TokenSet{}, fmt.Errorf("oauth: decode token response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || tr.AccessToken == "" {
		msg := tr.Error
		if tr.ErrorDesc != "" {
			msg = strings.TrimPrefix(msg+": "+tr.ErrorDesc, ": ")
		}
		if msg == "" {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return TokenSet{}, fmt.Errorf("oauth: token request failed: %s", msg)
	}
	ts := TokenSet{AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken}
	if tr.ExpiresIn > 0 {
		ts.ExpiresAt = now.Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return ts, nil
}

// ExchangeCode swaps an authorization code (+ PKCE verifier) for tokens.
func ExchangeCode(ctx context.Context, hc *http.Client, code, verifier, state string, now time.Time) (TokenSet, error) {
	return postToken(ctx, hc, map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  oauthRedirectURI,
		"client_id":     oauthClientID,
		"code_verifier": verifier,
		"state":         state,
	}, now)
}

// RefreshToken exchanges a refresh token for a fresh access token.
func RefreshToken(ctx context.Context, hc *http.Client, refresh string, now time.Time) (TokenSet, error) {
	return postToken(ctx, hc, map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refresh,
		"client_id":     oauthClientID,
	}, now)
}

// tokenExpired reports whether an access token expiring at expiresAt should be
// refreshed as of now (with a small skew). A zero expiresAt is treated as
// non-expiring (some grants omit expires_in).
func tokenExpired(expiresAt, now time.Time) bool {
	if expiresAt.IsZero() {
		return false
	}
	return !now.Before(expiresAt.Add(-refreshSkew))
}
