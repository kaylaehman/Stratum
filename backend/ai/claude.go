package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// claudeEndpoint is the Anthropic Messages API.
const claudeEndpoint = "https://api.anthropic.com/v1/messages"

// claudeAPIVersion is the required anthropic-version header value.
const claudeAPIVersion = "2023-06-01"

// DefaultClaudeModel is used when no model is configured.
const DefaultClaudeModel = "claude-sonnet-4-6"

// Claude talks to the Anthropic Messages API with an API key.
type Claude struct {
	apiKey string
	model  string
	http   *http.Client
}

// NewClaude builds a Claude provider. model defaults to DefaultClaudeModel.
func NewClaude(apiKey, model string, hc *http.Client) *Claude {
	if model == "" {
		model = DefaultClaudeModel
	}
	return &Claude{apiKey: apiKey, model: model, http: hc}
}

func (c *Claude) Kind() string { return ProviderClaude }

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
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
		System:    req.System,
		Messages:  []claudeMessage{{Role: "user", Content: req.Prompt}},
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeEndpoint, bytes.NewReader(body))
	if err != nil {
		return AskResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return AskResponse{}, fmt.Errorf("claude: request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	var out claudeResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return AskResponse{}, fmt.Errorf("claude: decode (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		if out.Error != nil {
			// Never echo the request (it could contain the key in a header dump);
			// surface only the provider's error category/message.
			return AskResponse{}, fmt.Errorf("claude: %s: %s", out.Error.Type, out.Error.Message)
		}
		return AskResponse{}, fmt.Errorf("claude: status %d", resp.StatusCode)
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
