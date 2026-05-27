package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
)

type secretKeyView struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type secretGroupView struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Secrets     []secretKeyView `json:"secrets"`
}

// ListSecrets returns all groups with their secret KEY NAMES only (never
// values). Admin-gated.
func (h *Handlers) ListSecrets(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	groups, err := h.Store.ListSecretGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]secretGroupView, 0, len(groups))
	for _, g := range groups {
		secrets, _ := h.Store.ListSecretKeysByGroup(r.Context(), g.ID) // id+key only; blob never loaded
		keys := make([]secretKeyView, len(secrets))
		for i, s := range secrets {
			keys[i] = secretKeyView{ID: s.ID, Key: s.Key}
		}
		out = append(out, secretGroupView{ID: g.ID, Name: g.Name, Description: g.Description, Secrets: keys})
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": out})
}

type groupBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateSecretGroup adds a group. Admin-gated + audited.
func (h *Handlers) CreateSecretGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var body groupBody
	if err := decodeJSON(r, &body); err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	g := db.SecretGroup{ID: uuid.NewString(), Name: body.Name, Description: body.Description}
	if err := h.Store.CreateSecretGroup(r.Context(), g); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	auditSecret(r, activity.ActionSecretGroupCreate, g.ID, map[string]string{"name": g.Name})
	writeJSON(w, http.StatusCreated, secretGroupView{ID: g.ID, Name: g.Name, Description: g.Description, Secrets: []secretKeyView{}})
}

// DeleteSecretGroup removes a group + its secrets. Admin-gated + audited.
func (h *Handlers) DeleteSecretGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteSecretGroup(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditSecret(r, activity.ActionSecretGroupDelete, id, nil)
	w.WriteHeader(http.StatusNoContent)
}

type setSecretBody struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SetSecret stores (seals) a secret under a group. Admin-gated + audited (the
// value is never logged).
func (h *Handlers) SetSecret(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	groupID := chi.URLParam(r, "id")
	var body setSecretBody
	if err := decodeJSON(r, &body); err != nil || body.Key == "" {
		writeError(w, http.StatusBadRequest, "key_required")
		return
	}
	if err := h.Secrets.SetSecret(r.Context(), groupID, body.Key, body.Value); err != nil {
		writeError(w, http.StatusInternalServerError, "set_failed")
		return
	}
	auditSecret(r, activity.ActionSecretSet, groupID, map[string]string{"key": body.Key})
	w.WriteHeader(http.StatusNoContent)
}

type importBody struct {
	Env string `json:"env"`
}

// ImportSecrets bulk-imports KEY=VALUE pairs from .env text. Admin-gated +
// audited (only the count is logged, never values).
func (h *Handlers) ImportSecrets(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	groupID := chi.URLParam(r, "id")
	var body importBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if len(body.Env) > 1<<20 { // cap .env import at 1 MiB to bound the O(n) write fan-out
		writeError(w, http.StatusBadRequest, "env_too_large")
		return
	}
	n, err := h.Secrets.ImportEnv(r.Context(), groupID, body.Env)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import_failed")
		return
	}
	auditSecret(r, activity.ActionSecretImport, groupID, map[string]string{"count": strconv.Itoa(n)})
	writeJSON(w, http.StatusOK, map[string]any{"imported": n})
}

// DeleteSecret removes one secret. Admin-gated + audited.
func (h *Handlers) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteSecret(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditSecret(r, activity.ActionSecretDelete, id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// RevealSecret decrypts and returns one secret's value. Admin-gated + audited
// (the reveal is logged with the key name, never the value).
func (h *Handlers) RevealSecret(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	key, value, err := h.Secrets.Reveal(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reveal_failed")
		return
	}
	auditSecret(r, activity.ActionSecretReveal, id, map[string]string{"key": key})
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "value": value})
}

func auditSecret(r *http.Request, action, id string, detail map[string]string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetSecret)
		e.TargetID = &id
		if detail != nil {
			e.Detail = detail
		}
	}
}
