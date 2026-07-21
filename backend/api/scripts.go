package api

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/db"
)

const scriptRunTimeout = 5 * time.Minute

func scriptView(s db.Script) map[string]any {
	return map[string]any{
		"id": s.ID, "name": s.Name, "description": s.Description, "content": s.Content,
	}
}

// ListScripts returns the saved scripts (read; authenticated).
func (h *Handlers) ListScripts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Store.ListScripts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]map[string]any, len(rows))
	for i, s := range rows {
		out[i] = scriptView(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"scripts": out})
}

type scriptBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

func (b scriptBody) valid() bool { return b.Name != "" && b.Content != "" }

// CreateScript saves a script. Admin-gated + audited.
func (h *Handlers) CreateScript(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var body scriptBody
	if err := decodeJSON(r, &body); err != nil || !body.valid() {
		writeError(w, http.StatusBadRequest, "name_and_content_required")
		return
	}
	s := db.Script{ID: uuid.NewString(), Name: body.Name, Description: body.Description, Content: body.Content}
	if err := h.Store.CreateScript(r.Context(), s); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	auditScript(r, activity.ActionScriptCreate, s.ID, s.Name)
	writeJSON(w, http.StatusCreated, scriptView(s))
}

// UpdateScript edits a script. Admin-gated + audited.
func (h *Handlers) UpdateScript(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var body scriptBody
	if err := decodeJSON(r, &body); err != nil || !body.valid() {
		writeError(w, http.StatusBadRequest, "name_and_content_required")
		return
	}
	s := db.Script{ID: id, Name: body.Name, Description: body.Description, Content: body.Content}
	if err := h.Store.UpdateScript(r.Context(), s); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	auditScript(r, activity.ActionScriptUpdate, id, body.Name)
	writeJSON(w, http.StatusOK, scriptView(s))
}

// DeleteScript removes a script. Admin-gated + audited.
func (h *Handlers) DeleteScript(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteScript(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditScript(r, activity.ActionScriptDelete, id, "")
	w.WriteHeader(http.StatusNoContent)
}

type runScriptBody struct {
	NodeIDs []string `json:"node_ids"`
}

type scriptRunResult struct {
	NodeID string `json:"node_id"`
	OK     bool   `json:"ok"`
	Output string `json:"output"`
}

// RunScript runs a saved script on the selected nodes over SSH (per-node,
// combined stdout+stderr). Admin-gated + audited. Hosts only — never containers.
func (h *Handlers) RunScript(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	// Running an arbitrary script as root on hosts is high-risk — require a fresh
	// TOTP step-up (fail-closed when the feature is on), matching FSDelete/DeleteNode.
	// The frontend apiFetch 428 interceptor drives the challenge + retry.
	if !h.requireStepUp(w, r) {
		return
	}
	s, err := h.Store.GetScript(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var body runScriptBody
	if err := decodeJSON(r, &body); err != nil || len(body.NodeIDs) == 0 {
		writeError(w, http.StatusBadRequest, "node_ids_required")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionScriptRun
		e.TargetType = ptr(activity.TargetScript)
		e.TargetID = &s.ID
		e.Detail = map[string]string{"name": s.Name, "node_count": strconv.Itoa(len(body.NodeIDs))}
	}

	cmd := runCommand(s.Content)
	results := make([]scriptRunResult, 0, len(body.NodeIDs))
	for _, nid := range body.NodeIDs {
		ctx, cancel := context.WithTimeout(r.Context(), scriptRunTimeout)
		out, err := h.Files.Exec(ctx, nid, "sh", "-c", cmd)
		cancel()
		results = append(results, scriptRunResult{NodeID: nid, OK: err == nil, Output: out})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// runCommand builds an injection-safe one-liner that base64-decodes the script
// content and pipes it to `sh` (combined stdout+stderr). The base64 alphabet is
// safe inside single quotes, so the admin-authored content can't break out.
func runCommand(content string) string {
	b64 := base64.StdEncoding.EncodeToString([]byte(content))
	return "printf '%s' '" + b64 + "' | base64 -d | sh 2>&1"
}

func auditScript(r *http.Request, action, id, name string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetScript)
		e.TargetID = &id
		if name != "" {
			e.Detail = map[string]string{"name": name}
		}
	}
}
