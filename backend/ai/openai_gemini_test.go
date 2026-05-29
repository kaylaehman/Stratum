package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAI_Ask(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi there"}}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`))
	}))
	defer srv.Close()
	old := openaiEndpoint
	openaiEndpoint = srv.URL
	defer func() { openaiEndpoint = old }()

	c := NewOpenAI("sk-test", "", srv.Client())
	if c.model != DefaultOpenAIModel {
		t.Errorf("default model = %q, want %q", c.model, DefaultOpenAIModel)
	}
	resp, err := c.Ask(context.Background(), AskRequest{System: "sys", Prompt: "q"})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if resp.Answer != "hi there" || resp.InputTokens != 7 || resp.OutputTokens != 3 {
		t.Errorf("resp = %+v", resp)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"role":"system"`) || !strings.Contains(gotBody, `"role":"user"`) {
		t.Errorf("body missing system/user messages: %s", gotBody)
	}
	if c.Kind() != ProviderOpenAI {
		t.Errorf("Kind = %q", c.Kind())
	}
}

func TestOpenAI_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"bad key"}}`))
	}))
	defer srv.Close()
	old := openaiEndpoint
	openaiEndpoint = srv.URL
	defer func() { openaiEndpoint = old }()

	_, err := NewOpenAI("x", "", srv.Client()).Ask(context.Background(), AskRequest{Prompt: "q"})
	if err == nil || !strings.Contains(err.Error(), "bad key") {
		t.Errorf("want error with message, got %v", err)
	}
}

func TestGemini_Ask(t *testing.T) {
	var gotPath, gotBody, gotKeyHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotKeyHeader = r.Header.Get("x-goog-api-key")
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2}}`))
	}))
	defer srv.Close()
	old := geminiBaseURL
	geminiBaseURL = srv.URL
	defer func() { geminiBaseURL = old }()

	c := NewGemini("AIza-test", "", srv.Client())
	if c.model != DefaultGeminiModel {
		t.Errorf("default model = %q, want %q", c.model, DefaultGeminiModel)
	}
	resp, err := c.Ask(context.Background(), AskRequest{System: "sys", Prompt: "q"})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if resp.Answer != "hello" || resp.InputTokens != 5 || resp.OutputTokens != 2 {
		t.Errorf("resp = %+v", resp)
	}
	if !strings.Contains(gotPath, DefaultGeminiModel+":generateContent") {
		t.Errorf("request path missing model: %s", gotPath)
	}
	// The key must travel in the header, never the URL (no leak via url.Error).
	if gotKeyHeader != "AIza-test" {
		t.Errorf("x-goog-api-key header = %q, want AIza-test", gotKeyHeader)
	}
	if strings.Contains(gotPath, "AIza-test") {
		t.Errorf("API key must NOT appear in the URL: %s", gotPath)
	}
	if !strings.Contains(gotBody, `"system_instruction"`) {
		t.Errorf("body missing system_instruction: %s", gotBody)
	}
	if c.Kind() != ProviderGemini {
		t.Errorf("Kind = %q", c.Kind())
	}
}
