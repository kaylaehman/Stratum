package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
)

// ListRunbooks returns all saved runbooks (read-only).
func (h *Handlers) ListRunbooks(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.ListRunbooks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if list == nil {
		list = []db.Runbook{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"runbooks": list})
}

type runbookRequest struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	TriggerConditions []string `json:"trigger_conditions"`
	Steps             []string `json:"steps"`
	RequiresApproval  bool     `json:"requires_approval"`
}

// CreateRunbook saves a new runbook (operator+). Audited.
func (h *Handlers) CreateRunbook(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	var req runbookRequest
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	rb := db.Runbook{
		ID: uuid.NewString(), Name: req.Name, Description: req.Description,
		TriggerConditions: norm(req.TriggerConditions), Steps: norm(req.Steps),
		RequiresApproval: req.RequiresApproval, CreatedAt: time.Now(),
	}
	if err := h.Store.CreateRunbook(r.Context(), rb); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	auditRunbook(r, activity.ActionRunbookCreate, rb.ID)
	writeJSON(w, http.StatusCreated, rb)
}

// UpdateRunbook edits a runbook (operator+). Audited.
func (h *Handlers) UpdateRunbook(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	cur, err := h.Store.GetRunbook(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req runbookRequest
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	cur.Name, cur.Description = req.Name, req.Description
	cur.TriggerConditions, cur.Steps = norm(req.TriggerConditions), norm(req.Steps)
	cur.RequiresApproval = req.RequiresApproval
	if err := h.Store.UpdateRunbook(r.Context(), cur); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	auditRunbook(r, activity.ActionRunbookUpdate, id)
	writeJSON(w, http.StatusOK, cur)
}

// DeleteRunbook removes a runbook (operator+). Audited.
func (h *Handlers) DeleteRunbook(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteRunbook(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditRunbook(r, activity.ActionRunbookDelete, id)
	w.WriteHeader(http.StatusNoContent)
}

func auditRunbook(r *http.Request, action, id string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetAI)
		e.TargetID = &id
	}
}

// norm drops empty/blank entries from a string list.
func norm(in []string) []string {
	out := []string{}
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
