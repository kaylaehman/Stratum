package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/sso"
)

// ListSSO returns all per-container SSO configs (admin). Client secrets are
// never included (only has_client_secret).
func (h *Handlers) ListSSO(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	list, err := h.SSO.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if list == nil {
		list = []db.SSOConfig{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"configs": list})
}

type ssoRequest struct {
	NodeID              string   `json:"node_id"`
	ContainerName       string   `json:"container_name"`
	Enabled             bool     `json:"enabled"`
	Method              string   `json:"method"`
	ProviderURL         string   `json:"provider_url"`
	ClientID            string   `json:"client_id"`
	ClientSecret        *string  `json:"client_secret"` // nil keep, "" clear, value set
	AllowedGroups       []string `json:"allowed_groups"`
	SessionDurationSecs int      `json:"session_duration_secs"`
}

// UpsertSSO stores a container's SSO config (admin). Audited. The client secret
// is sealed and never echoed.
func (h *Handlers) UpsertSSO(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req ssoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	cfg, err := h.SSO.Upsert(r.Context(), db.SSOConfig{
		NodeID: req.NodeID, ContainerName: req.ContainerName, Enabled: req.Enabled,
		Method: req.Method, ProviderURL: req.ProviderURL, ClientID: req.ClientID,
		AllowedGroups: req.AllowedGroups, SessionDurationSecs: req.SessionDurationSecs,
	}, req.ClientSecret)
	if errors.Is(err, sso.ErrInvalid) {
		writeError(w, http.StatusBadRequest, "invalid_config")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionSSOConfig
		e.TargetType = ptr(activity.TargetContainer)
		id := cfg.ID
		e.TargetID = &id
	}
	writeJSON(w, http.StatusOK, cfg)
}

// DeleteSSO removes a container's SSO config (admin). Audited.
func (h *Handlers) DeleteSSO(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.SSO.Delete(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionSSOConfig
		e.TargetType = ptr(activity.TargetContainer)
		e.TargetID = &id
	}
	w.WriteHeader(http.StatusNoContent)
}
