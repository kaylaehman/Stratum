package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/configversion"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/middleware"
)

// ConfigVersionHistory handles GET /api/nodes/{id}/configversions?path=
// Returns the version history for the given (node, path), newest first.
// Content is included in each row so the caller can diff locally.
func (h *Handlers) ConfigVersionHistory(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if h.ConfigVersions == nil {
		writeError(w, http.StatusNotImplemented, "feature_disabled")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path_required")
		return
	}
	history, err := h.ConfigVersions.History(r.Context(), chi.URLParam(r, "id"), path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": history})
}

type snapshotBody struct {
	Path string `json:"path"`
}

// ConfigVersionSnapshot handles POST /api/nodes/{id}/configversions/snapshot
// Takes a manual snapshot of the current on-disk content of the given path.
func (h *Handlers) ConfigVersionSnapshot(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if h.ConfigVersions == nil {
		writeError(w, http.StatusNotImplemented, "feature_disabled")
		return
	}
	var body snapshotBody
	if err := decodeJSON(r, &body); err != nil || body.Path == "" {
		writeError(w, http.StatusBadRequest, "path_required")
		return
	}
	u, _ := middleware.UserFromContext(r.Context())
	v, err := h.ConfigVersions.Snapshot(r.Context(), chi.URLParam(r, "id"), body.Path, u.ID)
	if errors.Is(err, configversion.ErrContentTooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	}
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "fs_error")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionConfigSnapshot
		e.TargetType = ptr("config_version")
		e.TargetID = &v.ID
	}
	writeJSON(w, http.StatusCreated, v)
}

type revertBody struct {
	VersionID string `json:"version_id"`
}

// ConfigVersionRevert handles POST /api/nodes/{id}/configversions/revert
// Writes a chosen snapshot's content back to disk. Requires admin + step-up.
// Audited as config.revert.
func (h *Handlers) ConfigVersionRevert(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	if h.ConfigVersions == nil {
		writeError(w, http.StatusNotImplemented, "feature_disabled")
		return
	}
	var body revertBody
	if err := decodeJSON(r, &body); err != nil || body.VersionID == "" {
		writeError(w, http.StatusBadRequest, "version_id_required")
		return
	}
	v, err := h.ConfigVersions.Revert(r.Context(), chi.URLParam(r, "id"), body.VersionID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "revert_error")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionConfigRevert
		e.TargetType = ptr("config_version")
		e.TargetID = &v.ID
		e.Detail = map[string]any{"node_id": v.NodeID, "path": v.Path, "version_id": v.ID}
	}
	writeJSON(w, http.StatusOK, v)
}

// ConfigVersionDrift handles GET /api/nodes/{id}/configversions/drift?path=
// Compares the current on-disk content to the latest known-good snapshot.
func (h *Handlers) ConfigVersionDrift(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if h.ConfigVersions == nil {
		writeError(w, http.StatusNotImplemented, "feature_disabled")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path_required")
		return
	}
	result, err := h.ConfigVersions.Drift(r.Context(), chi.URLParam(r, "id"), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "drift_error")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
