package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
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

// AIOAuthStart begins the Claude OAuth sign-in (admin). Returns the authorize
// URL to open in a browser plus the PKCE verifier + state the client echoes
// back to /exchange. No state is persisted server-side (stateless PKCE).
func (h *Handlers) AIOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	authorizeURL, verifier, state, err := h.AI.OAuthStart()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oauth_start_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authorize_url": authorizeURL,
		"verifier":      verifier,
		"state":         state,
	})
}

type aiOAuthExchangeRequest struct {
	Code     string `json:"code"`
	Verifier string `json:"verifier"`
	State    string `json:"state"`
}

// oauthExchangeTimeout bounds the token round-trip to Anthropic.
const oauthExchangeTimeout = 30 * time.Second

// AIOAuthExchange swaps the pasted authorization code for tokens (admin).
// Audited; tokens are sealed before storage and never echoed.
func (h *Handlers) AIOAuthExchange(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req aiOAuthExchangeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), oauthExchangeTimeout)
	defer cancel()
	if err := h.AI.OAuthExchange(ctx, req.Code, req.Verifier, req.State); errors.Is(err, ai.ErrInvalidConfig) {
		writeError(w, http.StatusBadRequest, "invalid_code")
		return
	} else if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("ai oauth exchange failed", "error", err)
		}
		writeError(w, http.StatusBadGateway, "oauth_exchange_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionAIConfig
		e.TargetType = ptr(activity.TargetAI)
		e.Detail = map[string]string{"provider": ai.ProviderClaudeOAuth, "event": "oauth_connect"}
	}
	writeJSON(w, http.StatusOK, h.AI.Config(r.Context()))
}

// AIOAuthDisconnect clears the stored Claude OAuth tokens (admin). Audited.
func (h *Handlers) AIOAuthDisconnect(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := h.AI.OAuthDisconnect(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionAIConfig
		e.TargetType = ptr(activity.TargetAI)
		e.Detail = map[string]string{"provider": ai.ProviderClaudeOAuth, "event": "oauth_disconnect"}
	}
	writeJSON(w, http.StatusOK, h.AI.Config(r.Context()))
}

type aiAskRequest struct {
	Task    string `json:"task"`
	Prompt  string `json:"prompt"`
	Context string `json:"context"`
	// Scope/ScopeID optionally bind the question to a node or container so the
	// assistant's relevant agent-memory (Feature F9) is injected as context.
	Scope   string `json:"scope"`
	ScopeID string `json:"scope_id"`
}

// memoryContext builds a short "known facts" block from confirmed agent
// memories: always the global scope, plus the request's node/container scope.
func (h *Handlers) memoryContext(r *http.Request, scope, scopeID string) string {
	var facts []string
	collect := func(sc, id string) {
		mems, err := h.Store.ListAgentMemory(r.Context(), sc, id, true)
		if err != nil {
			return
		}
		for _, m := range mems {
			facts = append(facts, "- ("+m.Scope+") "+m.Key+": "+m.Value)
		}
	}
	collect("global", "")
	if validMemoryScope(scope) && scope != "global" && scopeID != "" {
		collect(scope, scopeID)
	}
	if len(facts) == 0 {
		return ""
	}
	return "Known facts the operator has recorded:\n" + strings.Join(facts, "\n")
}

// runbookContext lists the saved runbooks (name — description; triggers) so the
// assistant can suggest following an established procedure. Steps are omitted to
// bound prompt size; the assistant can ask the operator to open the runbook.
func (h *Handlers) runbookContext(r *http.Request) string {
	books, err := h.Store.ListRunbooks(r.Context())
	if err != nil || len(books) == 0 {
		return ""
	}
	var lines []string
	for _, b := range books {
		line := "- " + b.Name
		if b.Description != "" {
			line += " — " + b.Description
		}
		if len(b.TriggerConditions) > 0 {
			line += " (triggers: " + strings.Join(b.TriggerConditions, "; ") + ")"
		}
		lines = append(lines, line)
	}
	return "Saved runbooks the operator maintains (suggest one when its trigger matches):\n" + strings.Join(lines, "\n")
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

	contextText := req.Context
	if mem := h.memoryContext(r, req.Scope, req.ScopeID); mem != "" {
		contextText = mem + "\n\n" + contextText
	}
	if rb := h.runbookContext(r); rb != "" {
		contextText = rb + "\n\n" + contextText
	}

	ctx, cancel := context.WithTimeout(r.Context(), aiAskTimeout)
	defer cancel()
	resp, provider, err := h.AI.Ask(ctx, req.Task, req.Prompt, contextText)
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
