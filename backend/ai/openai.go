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

// defaultOpenAIBaseURL is the real OpenAI API. Overridable per-config so the
// provider can target any OpenAI-compatible endpoint (claude-max-api-proxy,
// LiteLLM, vLLM, OpenRouter, …).
const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// DefaultOpenAIModel is used when no model is configured.
const DefaultOpenAIModel = "gpt-4o-mini"

// OpenAI talks to an OpenAI-compatible Chat Completions API.
type OpenAI struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

// NewOpenAI builds an OpenAI provider. model defaults to DefaultOpenAIModel;
// baseURL defaults to the real OpenAI API.
func NewOpenAI(apiKey, model, baseURL string, hc *http.Client) *OpenAI {
	if model == "" {
		model = DefaultOpenAIModel
	}
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return &OpenAI{apiKey: apiKey, model: model, baseURL: strings.TrimRight(baseURL, "/"), http: hc}
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return AskResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Some OpenAI-compatible proxies (e.g. claude-max-api-proxy) need no key;
	// only send auth when one is configured.
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return AskResponse{}, &ProviderError{Provider: "openai", Type: "network_error", Message: "couldn't reach the OpenAI API (check the Stratum host's outbound network, DNS, and TLS)"}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	if resp.StatusCode != http.StatusOK {
		var out openaiResponse
		_ = json.Unmarshal(raw, &out)
		if out.Error != nil {
			return AskResponse{}, &ProviderError{Provider: "openai", Type: out.Error.Type, Message: out.Error.Message}
		}
		return AskResponse{}, &ProviderError{Provider: "openai", Type: fmt.Sprintf("status_%d", resp.StatusCode), Message: errSnippet(raw)}
	}

	var out openaiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return AskResponse{}, fmt.Errorf("openai: decode: %w", err)
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
