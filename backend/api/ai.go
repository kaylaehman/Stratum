package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/ai"
)

// AIConfigGet returns the non-secret AI provider configuration (admin).
func (h *Handlers) AIConfigGet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, h.AI.Config(r.Context()))
}

// AIConfigSet updates the AI provider configuration (admin). Audited; the API
// key is never logged or echoed back.
func (h *Handlers) AIConfigSet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var u ai.ConfigUpdate
	if err := decodeJSON(r, &u); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if err := h.AI.SetConfig(r.Context(), u); errors.Is(err, ai.ErrInvalidConfig) {
		writeError(w, http.StatusBadRequest, "invalid_config")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionAIConfig
		e.TargetType = ptr(activity.TargetAI)
		// Record the provider only — never the key.
		e.Detail = map[string]string{"provider": u.Provider}
	}
	writeJSON(w, http.StatusOK, h.AI.Config(r.Context()))
}

type aiAskRequest struct {
	Task    string `json:"task"`
	Prompt  string `json:"prompt"`
	Context string `json:"context"`
}

// aiAskTimeout bounds a single assistant call (a local model can be slow).
const aiAskTimeout = 2 * time.Minute

// AIAsk answers a single-turn question via the configured provider. Gated to
// operator+ because it egresses caller-supplied infrastructure context (logs,
// inspect, config) to an external/local LLM — not something a read-only viewer
// should do. Audited (who asked, which provider, token usage — never the
// prompt/context, which may be large or sensitive).
func (h *Handlers) AIAsk(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	var req aiAskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt_required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), aiAskTimeout)
	defer cancel()
	resp, provider, err := h.AI.Ask(ctx, req.Task, req.Prompt, req.Context)
	if errors.Is(err, ai.ErrNotConfigured) {
		writeError(w, http.StatusBadRequest, "ai_not_configured")
		return
	} else if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("ai ask failed", "error", err)
		}
		writeError(w, http.StatusBadGateway, "ai_request_failed")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionAIAsk
		e.TargetType = ptr(activity.TargetAI)
		e.Detail = map[string]string{
			"task":          req.Task,
			"provider":      provider,
			"input_tokens":  strconv.Itoa(resp.InputTokens),
			"output_tokens": strconv.Itoa(resp.OutputTokens),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"answer":        resp.Answer,
		"provider":      provider,
		"input_tokens":  resp.InputTokens,
		"output_tokens": resp.OutputTokens,
	})
}
