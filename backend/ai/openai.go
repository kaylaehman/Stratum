package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openaiEndpoint is the Chat Completions API. A var (not const) so tests can
// point it at a stub server.
var openaiEndpoint = "https://api.openai.com/v1/chat/completions"

// DefaultOpenAIModel is used when no model is configured.
const DefaultOpenAIModel = "gpt-4o-mini"

// OpenAI talks to the OpenAI Chat Completions API with an API key.
type OpenAI struct {
	apiKey string
	model  string
	http   *http.Client
}

// NewOpenAI builds an OpenAI provider. model defaults to DefaultOpenAIModel.
func NewOpenAI(apiKey, model string, hc *http.Client) *OpenAI {
	if model == "" {
		model = DefaultOpenAIModel
	}
	return &OpenAI{apiKey: apiKey, model: model, http: hc}
}

func (c *OpenAI) Kind() string { return ProviderOpenAI }

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiRequest struct {
	Model     string          `json:"model"`
	Messages  []openaiMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

type openaiResponse struct {
	Choices []struct {
		Message openaiMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *OpenAI) Ask(ctx context.Context, req AskRequest) (AskResponse, error) {
	maxTok := req.MaxTokens
	if maxTok <= 0 {
		maxTok = defaultMaxTokens
	}
	msgs := []openaiMessage{}
	if req.System != "" {
		msgs = append(msgs, openaiMessage{Role: "system", Content: req.System})
	}
	msgs = append(msgs, openaiMessage{Role: "user", Content: req.Prompt})

	body, _ := json.Marshal(openaiRequest{Model: c.model, Messages: msgs, MaxTokens: maxTok})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiEndpoint, bytes.NewReader(body))
	if err != nil {
		return AskResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return AskResponse{}, fmt.Errorf("openai: request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	var out openaiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return AskResponse{}, fmt.Errorf("openai: decode (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		if out.Error != nil {
			return AskResponse{}, &ProviderError{Provider: "openai", Type: out.Error.Type, Message: out.Error.Message}
		}
		return AskResponse{}, fmt.Errorf("openai: status %d", resp.StatusCode)
	}
	var sb strings.Builder
	for _, ch := range out.Choices {
		sb.WriteString(ch.Message.Content)
	}
	return AskResponse{
		Answer:       strings.TrimSpace(sb.String()),
		InputTokens:  out.Usage.PromptTokens,
		OutputTokens: out.Usage.CompletionTokens,
	}, nil
}
