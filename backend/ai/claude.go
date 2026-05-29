package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// claudeEndpoint is the Anthropic Messages API. A var (not const) so tests can
// point it at a stub server.
var claudeEndpoint = "https://api.anthropic.com/v1/messages"

// claudeAPIVersion is the required anthropic-version header value.
const claudeAPIVersion = "2023-06-01"

// DefaultClaudeModel is used when no model is configured.
const DefaultClaudeModel = "claude-sonnet-4-6"

// claudeCodeIdentity must be the FIRST system block when authenticating with a
// Claude subscription OAuth token: Anthropic only accepts OAuth (oauth-2025-04-20)
// Messages requests whose system prompt identifies as Claude Code. Omitting it
// makes the API reject the request — so OAuth "connects" but every ask fails.
const claudeCodeIdentity = "You are Claude Code, Anthropic's official CLI for Claude."

// Claude talks to the Anthropic Messages API. It authenticates with either an
// API key (x-api-key) or an OAuth bearer token (Authorization: Bearer + the
// anthropic-beta oauth header) — exactly one is set.
type Claude struct {
	apiKey string // x-api-key mode
	bearer string // OAuth mode (Claude subscription token)
	model  string
	http   *http.Client
}

// NewClaude builds an API-key Claude provider. model defaults to DefaultClaudeModel.
func NewClaude(apiKey, model string, hc *http.Client) *Claude {
	if model == "" {
		model = DefaultClaudeModel
	}
	return &Claude{apiKey: apiKey, model: model, http: hc}
}

// NewClaudeOAuth builds a Claude provider authenticated with an OAuth bearer
// token (the "claude.ai -p" method) rather than an API key.
func NewClaudeOAuth(bearer, model string, hc *http.Client) *Claude {
	if model == "" {
		model = DefaultClaudeModel
	}
	return &Claude{bearer: bearer, model: model, http: hc}
}

func (c *Claude) Kind() string {
	if c.bearer != "" {
		return ProviderClaudeOAuth
	}
	return ProviderClaude
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeSystemBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    any             `json:"system,omitempty"` // string (API key) or []claudeSystemBlock (OAuth)
	Messages  []claudeMessage `json:"messages"`
}

// systemField returns the system value for the request: a plain string for the
// API-key path, or, for OAuth, an array whose first block is the required
// Claude Code identity followed by our actual system prompt.
func (c *Claude) systemField(system string) any {
	if c.bearer == "" {
		if system == "" {
			return nil
		}
		return system
	}
	blocks := []claudeSystemBlock{{Type: "text", Text: claudeCodeIdentity}}
	if system != "" {
		blocks = append(blocks, claudeSystemBlock{Type: "text", Text: system})
	}
	return blocks
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Claude) Ask(ctx context.Context, req AskRequest) (AskResponse, error) {
	maxTok := req.MaxTokens
	if maxTok <= 0 {
		maxTok = defaultMaxTokens
	}
	body, _ := json.Marshal(claudeRequest{
		Model:     c.model,
		MaxTokens: maxTok,
		System:    c.systemField(req.System),
		Messages:  []claudeMessage{{Role: "user", Content: req.Prompt}},
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeEndpoint, bytes.NewReader(body))
	if err != nil {
		return AskResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)
	if c.bearer != "" {
		// OAuth (subscription) token: Bearer auth + the required beta opt-in.
		httpReq.Header.Set("Authorization", "Bearer "+c.bearer)
		httpReq.Header.Set("anthropic-beta", oauthBetaHeader)
	} else {
		httpReq.Header.Set("x-api-key", c.apiKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return AskResponse{}, &ProviderError{Provider: "claude", Type: "network_error", Message: "couldn't reach the Anthropic API (check the Stratum host's outbound network, DNS, and TLS)"}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	if resp.StatusCode != http.StatusOK {
		var out claudeResponse
		_ = json.Unmarshal(raw, &out)
		if out.Error != nil {
			return AskResponse{}, &ProviderError{Provider: "claude", Type: out.Error.Type, Message: out.Error.Message}
		}
		// Non-standard error body — surface the status + the provider's response
		// snippet (their words, no request secrets).
		return AskResponse{}, &ProviderError{Provider: "claude", Type: fmt.Sprintf("status_%d", resp.StatusCode), Message: errSnippet(raw)}
	}

	var out claudeResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return AskResponse{}, fmt.Errorf("claude: decode: %w", err)
	}
	var sb bytes.Buffer
	for _, blk := range out.Content {
		if blk.Type == "text" {
			sb.WriteString(blk.Text)
		}
	}
	return AskResponse{
		Answer:       sb.String(),
		InputTokens:  out.Usage.InputTokens,
		OutputTokens: out.Usage.OutputTokens,
	}, nil
}
