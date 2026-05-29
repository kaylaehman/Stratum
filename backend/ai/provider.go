// Package ai implements the AI Assistant (Feature 31): a provider-agnostic
// "ask" capability used for log explanation, container diagnosis, and config
// questions. Three providers are supported — a local Ollama-compatible
// endpoint, the Claude (Anthropic) API key, and Claude OAuth (the "claude.ai
// -p" sign-in, which uses a subscription token instead of an API key). All
// outbound calls go through an injectable *http.Client so the providers are
// unit-testable without a network.
package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// errSnippet returns a short single-line snippet of a provider RESPONSE body.
// Safe to surface: it's the provider's own response text (their error wording),
// not our request — so it carries no API key or request URL.
func errSnippet(raw []byte) string {
	s := strings.Join(strings.Fields(string(raw)), " ")
	const max = 300
	if len(s) > max {
		s = s[:max] + "…"
	}
	if s == "" {
		return "(empty response body)"
	}
	return s
}

// ProviderError is a structured error built from an LLM provider's API error
// RESPONSE (its error type + message). These fields are host- and secret-free,
// so they are safe to surface to an operator. Transport/decode errors are NOT
// wrapped in this type — they may embed request URLs/keys and must never be
// returned to the client; callers surface only *ProviderError detail.
type ProviderError struct {
	Provider string
	Type     string
	Message  string
}

func (e *ProviderError) Error() string {
	if e.Type != "" {
		return fmt.Sprintf("%s: %s: %s", e.Provider, e.Type, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Provider, e.Message)
}

// Provider kinds.
const (
	ProviderOllama      = "ollama"
	ProviderClaude      = "claude"
	ProviderClaudeOAuth = "claude-oauth"
	ProviderOpenAI      = "openai"
	ProviderGemini      = "gemini"
)

// ErrNotConfigured is returned when no usable provider is configured.
var ErrNotConfigured = errors.New("ai: no provider configured")

// AskRequest is a single-turn question with an assembled system prompt.
type AskRequest struct {
	System    string // instructions + any retrieved context
	Prompt    string // the user's question
	MaxTokens int    // response cap; 0 => provider default
}

// AskResponse is a provider's answer plus a best-effort token estimate.
type AskResponse struct {
	Answer      string `json:"answer"`
	InputTokens int    `json:"input_tokens,omitempty"`
	OutputTokens int   `json:"output_tokens,omitempty"`
}

// Provider is a chat/completion backend.
type Provider interface {
	// Ask sends a single-turn request and returns the answer. It must respect
	// ctx cancellation/timeout.
	Ask(ctx context.Context, req AskRequest) (AskResponse, error)
	// Kind returns the provider identifier (ProviderOllama|ProviderClaude).
	Kind() string
}

const defaultMaxTokens = 1024
