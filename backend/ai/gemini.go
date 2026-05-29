package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// geminiBaseURL is the Generative Language API base. A var (not const) so tests
// can point it at a stub server.
var geminiBaseURL = "https://generativelanguage.googleapis.com"

// DefaultGeminiModel is used when no model is configured.
const DefaultGeminiModel = "gemini-2.0-flash"

// Gemini talks to Google's Generative Language API (AI Studio) with an API key.
type Gemini struct {
	apiKey string
	model  string
	http   *http.Client
}

// NewGemini builds a Gemini provider. model defaults to DefaultGeminiModel.
func NewGemini(apiKey, model string, hc *http.Client) *Gemini {
	if model == "" {
		model = DefaultGeminiModel
	}
	return &Gemini{apiKey: apiKey, model: model, http: hc}
}

func (c *Gemini) Kind() string { return ProviderGemini }

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  struct {
		MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (c *Gemini) Ask(ctx context.Context, req AskRequest) (AskResponse, error) {
	maxTok := req.MaxTokens
	if maxTok <= 0 {
		maxTok = defaultMaxTokens
	}
	var body geminiRequest
	if req.System != "" {
		body.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: req.System}}}
	}
	body.Contents = []geminiContent{{Role: "user", Parts: []geminiPart{{Text: req.Prompt}}}}
	body.GenerationConfig.MaxOutputTokens = maxTok

	raw, _ := json.Marshal(body)
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimRight(geminiBaseURL, "/"), url.PathEscape(c.model), url.QueryEscape(c.apiKey))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return AskResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return AskResponse{}, fmt.Errorf("gemini: request: %w", err)
	}
	defer resp.Body.Close()
	respRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	var out geminiResponse
	if err := json.Unmarshal(respRaw, &out); err != nil {
		return AskResponse{}, fmt.Errorf("gemini: decode (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		if out.Error != nil {
			return AskResponse{}, fmt.Errorf("gemini: %s: %s", out.Error.Status, out.Error.Message)
		}
		return AskResponse{}, fmt.Errorf("gemini: status %d", resp.StatusCode)
	}
	var sb strings.Builder
	for _, cand := range out.Candidates {
		for _, p := range cand.Content.Parts {
			sb.WriteString(p.Text)
		}
	}
	return AskResponse{
		Answer:       strings.TrimSpace(sb.String()),
		InputTokens:  out.UsageMetadata.PromptTokenCount,
		OutputTokens: out.UsageMetadata.CandidatesTokenCount,
	}, nil
}
