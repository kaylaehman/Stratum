package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/automation"
	"github.com/KAE-Labs/stratum/backend/db"
)

// ListAutomations returns every automation with its merged catalog + DB state.
// Admin-gated in the handler (not at the router level because the read group
// doesn't carry an admin gate globally).
func (h *Handlers) ListAutomations(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if h.Automation == nil {
		writeError(w, http.StatusServiceUnavailable, "automation_not_configured")
		return
	}
	views, err := h.Automation.ListViews(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"automations": views})
}

// updateAutomationBody is the request body for PUT /api/automations/{key}.
type updateAutomationBody struct {
	Enabled         *bool          `json:"enabled"`
	IntervalSeconds *int           `json:"interval_seconds"`
	Config          map[string]any `json:"config"`
}

// UpdateAutomation lets an admin enable/disable an automation, adjust its
// interval, or update its config. Audited under automation.config.
func (h *Handlers) UpdateAutomation(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	key := chi.URLParam(r, "key")
	ent, ok := automation.CatalogEntry(key)
	if !ok {
		writeError(w, http.StatusNotFound, "automation_not_found")
		return
	}

	var body updateAutomationBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	// Load current effective state (using catalog entry as fallback when engine is nil).
	enabled := false
	intervalSeconds := ent.DefaultIntervalSeconds
	cfg := copyConfig(ent.DefaultConfig)

	if h.Automation != nil {
		current, err := h.Automation.GetView(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		enabled = current.Enabled
		intervalSeconds = current.IntervalSeconds
		cfg = current.Config
	}

	// Apply patch.
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	if body.IntervalSeconds != nil && *body.IntervalSeconds > 0 {
		intervalSeconds = *body.IntervalSeconds
	}
	if body.Config != nil {
		for k, v := range body.Config {
			cfg[k] = v
		}
	}
	cfgJSON, _ := json.Marshal(cfg)

	if err := h.Store.UpsertAutomation(r.Context(), key, enabled, intervalSeconds, string(cfgJSON)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionAutomationConfig
		e.TargetType = ptr(activity.TargetAutomation)
		e.TargetID = &key
		e.Detail = map[string]any{"enabled": enabled, "interval_seconds": intervalSeconds}
	}

	if h.Automation != nil {
		view, _ := h.Automation.GetView(r.Context(), key)
		writeJSON(w, http.StatusOK, view)
		return
	}
	// Engine not wired (test or disabled): return minimal view from catalog.
	writeJSON(w, http.StatusOK, map[string]any{
		"key":             key,
		"enabled":         enabled,
		"interval_seconds": intervalSeconds,
		"config":          cfg,
	})
}

// copyConfig makes a shallow clone of a config map (avoids mutating catalog defaults).
func copyConfig(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// RunAutomation manually triggers an automation. Operator-gated + audited.
func (h *Handlers) RunAutomation(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	key := chi.URLParam(r, "key")
	if _, ok := automation.CatalogEntry(key); !ok {
		writeError(w, http.StatusNotFound, "automation_not_found")
		return
	}
	if h.Automation == nil {
		writeError(w, http.StatusServiceUnavailable, "automation_not_configured")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionAutomationRun
		e.TargetType = ptr(activity.TargetAutomation)
		e.TargetID = &key
	}

	detail, err := h.Automation.RunNow(r.Context(), key)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "automation_not_found")
		return
	}
	result := "ok"
	if err != nil {
		result = "error"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key":    key,
		"status": result,
		"detail": detail,
	})
}
