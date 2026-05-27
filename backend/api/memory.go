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

// validMemoryScope reports whether scope is one of the allowed values.
func validMemoryScope(scope string) bool {
	switch scope {
	case "global", "node", "container":
		return true
	default:
		return false
	}
}

// ListMemory returns agent memories for a scope (?scope=&scope_id=). Read-only.
func (h *Handlers) ListMemory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	scope := q.Get("scope")
	if scope == "" {
		scope = "global"
	}
	if !validMemoryScope(scope) {
		writeError(w, http.StatusBadRequest, "invalid_scope")
		return
	}
	list, err := h.Store.ListAgentMemory(r.Context(), scope, q.Get("scope_id"), false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if list == nil {
		list = []db.AgentMemory{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": list})
}

type memoryCreateRequest struct {
	Scope   string `json:"scope"`
	ScopeID string `json:"scope_id"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

// CreateMemory adds a user-authored memory (operator+). Audited.
func (h *Handlers) CreateMemory(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	var req memoryCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if !validMemoryScope(req.Scope) || req.Key == "" || req.Value == "" {
		writeError(w, http.StatusBadRequest, "scope_key_value_required")
		return
	}
	if req.Scope == "global" {
		req.ScopeID = "" // normalize
	} else if req.ScopeID == "" {
		writeError(w, http.StatusBadRequest, "scope_id_required")
		return
	}
	m := db.AgentMemory{
		ID: uuid.NewString(), Scope: req.Scope, ScopeID: req.ScopeID,
		Key: req.Key, Value: req.Value, Source: "user", Confirmed: true,
		CreatedAt: time.Now(),
	}
	if err := h.Store.CreateAgentMemory(r.Context(), m); err != nil {
		writeError(w, http.StatusConflict, "memory_exists_or_failed")
		return
	}
	auditMemory(r, activity.ActionMemoryCreate, m.ID)
	writeJSON(w, http.StatusCreated, m)
}

type memoryUpdateRequest struct {
	Value     *string `json:"value"`
	Confirmed *bool   `json:"confirmed"`
}

// UpdateMemory edits a memory's value and/or confirms an AI-proposed one
// (operator+). Audited.
func (h *Handlers) UpdateMemory(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	cur, err := h.Store.GetAgentMemory(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req memoryUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.Value != nil {
		cur.Value = *req.Value
	}
	if req.Confirmed != nil {
		cur.Confirmed = *req.Confirmed
		if cur.Confirmed && cur.Source == "ai" {
			cur.Source = "user" // accepting an AI suggestion makes it user-owned
		}
	}
	if err := h.Store.UpdateAgentMemory(r.Context(), cur); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	auditMemory(r, activity.ActionMemoryUpdate, id)
	writeJSON(w, http.StatusOK, cur)
}

// DeleteMemory removes a memory (operator+). Audited.
func (h *Handlers) DeleteMemory(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteAgentMemory(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditMemory(r, activity.ActionMemoryDelete, id)
	w.WriteHeader(http.StatusNoContent)
}

func auditMemory(r *http.Request, action, id string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetAI)
		e.TargetID = &id
	}
}
