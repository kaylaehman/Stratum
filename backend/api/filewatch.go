package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/fs"
	"github.com/KAE-Labs/stratum/backend/middleware"
)

// ListWatches returns a node's file-watch config (admin).
func (h *Handlers) ListWatches(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	list, err := h.FileWatch.ListWatches(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if list == nil {
		list = []db.FileWatch{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"watches": list})
}

type addWatchRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

// AddWatch registers a watched path (admin). Audited.
func (h *Handlers) AddWatch(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	var req addWatchRequest
	if err := decodeJSON(r, &req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "path_required")
		return
	}
	// Reuse the fs path validation (absolute, no NUL/relative).
	if _, err := fs.ValidatePath(req.Path); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_path")
		return
	}
	by := ""
	if u, ok := middleware.UserFromContext(r.Context()); ok {
		by = u.Username
	}
	wch, err := h.FileWatch.AddWatch(r.Context(), nodeID, req.Path, req.Recursive, by)
	if err != nil {
		writeError(w, http.StatusConflict, "watch_exists_or_failed")
		return
	}
	auditWatch(r, activity.ActionWatchAdd, wch.ID)
	writeJSON(w, http.StatusCreated, wch)
}

// RemoveWatch deletes a watch (admin). Audited.
func (h *Handlers) RemoveWatch(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "watchID")
	if err := h.FileWatch.RemoveWatch(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditWatch(r, activity.ActionWatchDelete, id)
	w.WriteHeader(http.StatusNoContent)
}

// FileEvents lists recent file-change events (admin). ?node= filters; ?limit=.
func (h *Handlers) FileEvents(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	events, err := h.FileWatch.Events(r.Context(), q.Get("node"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if events == nil {
		events = []db.FileEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

// ScanWatches runs a manual file-change scan for a node (admin). Audited.
func (h *Handlers) ScanWatches(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()
	events, err := h.FileWatch.Scan(ctx, nodeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "scan_failed")
		return
	}
	auditWatch(r, activity.ActionWatchScan, nodeID)
	writeJSON(w, http.StatusOK, map[string]any{"detected": len(events)})
}

func auditWatch(r *http.Request, action, id string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &id
	}
}
