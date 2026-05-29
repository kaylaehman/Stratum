package ai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const claudeStubBody = `{"content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":3,"output_tokens":1}}`

func TestClaude_OAuthSendsClaudeCodeIdentity(t *testing.T) {
	var gotBody, gotAuth, gotBeta, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		gotAuth = r.Header.Get("Authorization")
		gotBeta = r.Header.Get("anthropic-beta")
		gotKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(claudeStubBody))
	}))
	defer srv.Close()
	old := claudeEndpoint
	claudeEndpoint = srv.URL
	defer func() { claudeEndpoint = old }()

	_, err := NewClaudeOAuth("oauth-tok", "", srv.Client()).
		Ask(context.Background(), AskRequest{System: "be helpful", Prompt: "q"})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if gotAuth != "Bearer oauth-tok" || gotBeta != oauthBetaHeader || gotKey != "" {
		t.Errorf("auth headers wrong: auth=%q beta=%q key=%q", gotAuth, gotBeta, gotKey)
	}
	// The required Claude Code identity must be present as a system block, and it
	// must precede our actual system prompt.
	if !strings.Contains(gotBody, claudeCodeIdentity) {
		t.Errorf("OAuth request missing Claude Code identity: %s", gotBody)
	}
	if i, j := strings.Index(gotBody, claudeCodeIdentity), strings.Index(gotBody, "be helpful"); i < 0 || j < 0 || i > j {
		t.Errorf("identity must come before the system prompt: %s", gotBody)
	}
}

func TestClaude_APIKeyDoesNotSpoofIdentity(t *testing.T) {
	var gotBody, gotKey, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		gotKey = r.Header.Get("x-api-key")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(claudeStubBody))
	}))
	defer srv.Close()
	old := claudeEndpoint
	claudeEndpoint = srv.URL
	defer func() { claudeEndpoint = old }()

	_, err := NewClaude("sk-ant-x", "", srv.Client()).
		Ask(context.Background(), AskRequest{System: "be helpful", Prompt: "q"})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if gotKey != "sk-ant-x" || gotAuth != "" {
		t.Errorf("API-key path headers wrong: key=%q auth=%q", gotKey, gotAuth)
	}
	if strings.Contains(gotBody, claudeCodeIdentity) {
		t.Errorf("API-key path must NOT inject the Claude Code identity: %s", gotBody)
	}
}
