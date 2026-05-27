package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/features"
)

// ListFeatures returns the feature-flag catalog with resolved enabled states.
// Available to any authenticated user (the frontend gates UI from it); only the
// toggle is admin-gated.
func (h *Handlers) ListFeatures(w http.ResponseWriter, r *http.Request) {
	flags, err := h.Features.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"features": flags})
}

type setFeatureRequest struct {
	Enabled bool `json:"enabled"`
}

// SetFeature toggles a feature flag (admin). Audited.
func (h *Handlers) SetFeature(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	key := chi.URLParam(r, "key")
	if !features.Valid(key) {
		writeError(w, http.StatusBadRequest, "unknown_feature")
		return
	}
	var req setFeatureRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if err := h.Features.Set(r.Context(), key, req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionFeatureToggle
		k := key
		e.TargetID = &k
		e.Detail = map[string]string{"enabled": boolStr(req.Enabled)}
	}
	flags, err := h.Features.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"features": flags})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
