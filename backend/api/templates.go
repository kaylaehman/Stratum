package api

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/templates"
)

type templateBody struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Tags        []string         `json:"tags"`
	ComposeYAML string           `json:"compose_yaml"`
	Variables   []db.TemplateVar `json:"variables"`
}

func (b templateBody) valid() bool { return b.Name != "" && b.ComposeYAML != "" }

func templateView(t db.Template) map[string]any {
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}
	vars := t.Variables
	if vars == nil {
		vars = []db.TemplateVar{}
	}
	return map[string]any{
		"id": t.ID, "name": t.Name, "description": t.Description, "tags": tags,
		"compose_yaml": t.ComposeYAML, "variables": vars, "version": t.Version,
	}
}

// ListTemplates returns all templates (read-only).
func (h *Handlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Store.ListTemplates(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]map[string]any, len(rows))
	for i, t := range rows {
		out[i] = templateView(t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}

// GetTemplate returns one template plus its version history.
func (h *Handlers) GetTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := h.Store.GetTemplate(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	versions, _ := h.Store.ListTemplateVersions(r.Context(), t.ID)
	if versions == nil {
		versions = []db.TemplateVersion{}
	}
	v := templateView(t)
	v["versions"] = versions
	writeJSON(w, http.StatusOK, v)
}

// CreateTemplate saves a new template at version 1. Admin-gated + audited.
func (h *Handlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var body templateBody
	if err := decodeJSON(r, &body); err != nil || !body.valid() {
		writeError(w, http.StatusBadRequest, "name_and_compose_required")
		return
	}
	t := db.Template{
		ID: uuid.NewString(), Name: body.Name, Description: body.Description, Tags: body.Tags,
		ComposeYAML: body.ComposeYAML, Variables: body.Variables, Version: 1,
	}
	if err := h.Store.CreateTemplate(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	_ = h.Store.AddTemplateVersion(r.Context(), t.ID, db.TemplateVersion{Version: 1, ComposeYAML: t.ComposeYAML, Variables: t.Variables})
	auditTemplate(r, activity.ActionTemplateCreate, t.ID, t.Name)
	writeJSON(w, http.StatusCreated, templateView(t))
}

// UpdateTemplate saves a new version of a template. Admin-gated + audited.
func (h *Handlers) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	cur, err := h.Store.GetTemplate(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var body templateBody
	if err := decodeJSON(r, &body); err != nil || !body.valid() {
		writeError(w, http.StatusBadRequest, "name_and_compose_required")
		return
	}
	next := db.Template{
		ID: id, Name: body.Name, Description: body.Description, Tags: body.Tags,
		ComposeYAML: body.ComposeYAML, Variables: body.Variables, Version: cur.Version + 1,
	}
	if err := h.Store.UpdateTemplate(r.Context(), next); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	_ = h.Store.AddTemplateVersion(r.Context(), id, db.TemplateVersion{Version: next.Version, ComposeYAML: next.ComposeYAML, Variables: next.Variables})
	auditTemplate(r, activity.ActionTemplateUpdate, id, next.Name)
	writeJSON(w, http.StatusOK, templateView(next))
}

// DeleteTemplate removes a template. Admin-gated + audited.
func (h *Handlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteTemplate(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditTemplate(r, activity.ActionTemplateDelete, id, "")
	w.WriteHeader(http.StatusNoContent)
}

type renderBody struct {
	Variables map[string]string `json:"variables"`
}

// RenderTemplate returns the template's compose with variables substituted, plus
// any unresolved tokens. Read-only string substitution (authenticated).
func (h *Handlers) RenderTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := h.Store.GetTemplate(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var body renderBody
	_ = decodeJSON(r, &body)
	rendered, unresolved := templates.Render(t.ComposeYAML, t.Variables, body.Variables)
	writeJSON(w, http.StatusOK, map[string]any{"rendered": rendered, "unresolved": unresolved})
}

type deployBody struct {
	NodeID    string            `json:"node_id"`
	Dir       string            `json:"dir"`
	Variables map[string]string `json:"variables"`
}

// DeployTemplate renders the template and brings the stack up on a node via
// `docker compose up -d` over SSH. Admin-gated + audited.
func (h *Handlers) DeployTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	t, err := h.Store.GetTemplate(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var body deployBody
	if err := decodeJSON(r, &body); err != nil || body.NodeID == "" {
		writeError(w, http.StatusBadRequest, "node_id_required")
		return
	}
	node, err := h.Store.GetNode(r.Context(), body.NodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if !caps.Docker {
		writeError(w, http.StatusConflict, "docker_not_available")
		return
	}

	rendered, unresolved := templates.Render(t.ComposeYAML, t.Variables, body.Variables)
	if len(unresolved) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unresolved_variables", "unresolved": unresolved})
		return
	}

	dir, ok := safeDeployDir(body.Dir, t.Name)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_dir")
		return
	}
	composePath := path.Join(dir, "docker-compose.yml")

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionTemplateDeploy
		e.TargetType = ptr(activity.TargetTemplate)
		e.TargetID = &t.ID
		e.Detail = map[string]string{"node_id": body.NodeID, "path": composePath, "template": t.Name}
	}

	if err := h.Files.Write(r.Context(), body.NodeID, composePath, []byte(rendered), nil); err != nil {
		writeError(w, http.StatusBadGateway, "write_failed")
		return
	}
	out, err := h.Files.Exec(r.Context(), body.NodeID, "docker", "compose", "-f", composePath, "up", "-d")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "compose_failed", "output": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": composePath, "output": out})
}

// deployDirAllowlist restricts where a stack may be deployed, so an admin can't
// clobber a host config file (e.g. /etc/cron.d/docker-compose.yml). Stacks live
// under conventional service-data roots.
var deployDirAllowlist = []string{"/opt", "/srv", "/home", "/var", "/mnt"}

// safeDeployDir validates/derives the absolute deploy directory. An empty dir
// defaults to /opt/stratum-stacks/<sanitized-name>. A supplied dir must be
// absolute, traversal-free, and under an allowlisted root.
func safeDeployDir(dir, name string) (string, bool) {
	if dir == "" {
		safe := strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
				return r
			default:
				return '-'
			}
		}, name)
		return "/opt/stratum-stacks/" + safe, true
	}
	if !strings.HasPrefix(dir, "/") || strings.Contains(dir, "..") {
		return "", false
	}
	clean := path.Clean(dir)
	for _, root := range deployDirAllowlist {
		if clean == root || strings.HasPrefix(clean, root+"/") {
			return clean, true
		}
	}
	return "", false
}

func auditTemplate(r *http.Request, action, id, name string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetTemplate)
		e.TargetID = &id
		if name != "" {
			e.Detail = map[string]string{"name": name}
		}
	}
}
