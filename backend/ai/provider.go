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
)

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
