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

// Ollama talks to a local Ollama-compatible /api/chat endpoint.
type Ollama struct {
	baseURL string
	model   string
	http    *http.Client
}

// NewOllama builds an Ollama provider. baseURL is e.g. "http://localhost:11434".
func NewOllama(baseURL, model string, hc *http.Client) *Ollama {
	return &Ollama{baseURL: strings.TrimRight(baseURL, "/"), model: model, http: hc}
}

func (o *Ollama) Kind() string { return ProviderOllama }

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	// Think disables a reasoning model's "thinking" channel so the answer lands
	// in message.content instead of message.thinking. Pointer so we can send an
	// explicit false; a no-op for non-reasoning models.
	Think *bool `json:"think,omitempty"`
}

// ollamaRespMessage is the assistant message in a /api/chat response. Reasoning
// models may place their answer in `thinking` and leave `content` empty.
type ollamaRespMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking"`
}

type ollamaChatResponse struct {
	Message         ollamaRespMessage `json:"message"`
	PromptEvalCount int               `json:"prompt_eval_count"`
	EvalCount       int               `json:"eval_count"`
	Error           string            `json:"error"`
}

func (o *Ollama) Ask(ctx context.Context, req AskRequest) (AskResponse, error) {
	msgs := []ollamaMessage{}
	if req.System != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: req.System})
	}
	msgs = append(msgs, ollamaMessage{Role: "user", Content: req.Prompt})

	think := false
	body, _ := json.Marshal(ollamaChatRequest{Model: o.model, Messages: msgs, Stream: false, Think: &think})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return AskResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.http.Do(httpReq)
	if err != nil {
		return AskResponse{}, fmt.Errorf("ollama: request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return AskResponse{}, fmt.Errorf("ollama: status %d", resp.StatusCode)
	}
	var out ollamaChatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return AskResponse{}, fmt.Errorf("ollama: decode: %w", err)
	}
	if out.Error != "" {
		return AskResponse{}, fmt.Errorf("ollama: %s", out.Error)
	}
	// Reasoning models may answer in `thinking` and leave `content` empty even
	// with think:false. Fall back to the thinking text so the user never sees a
	// blank reply.
	answer := strings.TrimSpace(out.Message.Content)
	if answer == "" {
		answer = strings.TrimSpace(out.Message.Thinking)
	}
	return AskResponse{
		Answer:       answer,
		InputTokens:  out.PromptEvalCount,
		OutputTokens: out.EvalCount,
	}, nil
}
