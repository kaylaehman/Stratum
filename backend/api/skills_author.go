package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/ai"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/middleware"
	"github.com/KAE-Labs/stratum/backend/skills"
)

// skillYAMLRequest is the body for create/update: the verbatim skill YAML.
type skillYAMLRequest struct {
	YAML string `json:"yaml"`
}

// CreateSkill validates and stores a user-authored skill, then merges it into
// the live library. Operator+; audited. The id is taken from the YAML; an id
// that collides with an existing skill (built-in or custom) is rejected — edits
// go through PUT.
func (h *Handlers) CreateSkill(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	if h.Skills == nil {
		writeError(w, http.StatusServiceUnavailable, "skills_unavailable")
		return
	}
	var req skillYAMLRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	s, err := skills.Parse([]byte(req.YAML))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_skill", "detail": err.Error()})
		return
	}
	if _, exists := h.Skills.Get(s.ID); exists {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "id_exists", "detail": "a skill with id " + s.ID + " already exists; edit it instead"})
		return
	}
	s.Source = skills.SourceCustom

	var createdBy string
	if u, ok := middleware.UserFromContext(r.Context()); ok {
		createdBy = u.ID
	}
	if err := h.Store.UpsertCustomSkill(r.Context(), db.CustomSkill{ID: s.ID, YAML: req.YAML, CreatedBy: createdBy}); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	h.Skills.Upsert(s)

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionSkillCreate
		e.TargetType = ptr(activity.TargetSkill)
		e.TargetID = ptr(s.ID)
		e.Detail = map[string]string{"id": s.ID, "name": s.Name}
	}
	writeJSON(w, http.StatusCreated, detail(s))
}

// UpdateSkill replaces the YAML of an existing custom skill. Operator+; audited.
// Built-in skills are read-only (403). The YAML's id must match the path id.
func (h *Handlers) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	if h.Skills == nil {
		writeError(w, http.StatusServiceUnavailable, "skills_unavailable")
		return
	}
	id := chi.URLParam(r, "id")
	existing, ok := h.Skills.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if existing.Source != skills.SourceCustom {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "builtin_readonly", "detail": "built-in skills cannot be edited; clone it into a new skill instead"})
		return
	}
	var req skillYAMLRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	s, err := skills.Parse([]byte(req.YAML))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_skill", "detail": err.Error()})
		return
	}
	if s.ID != id {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id_mismatch", "detail": "the skill id cannot be changed on edit"})
		return
	}
	s.Source = skills.SourceCustom
	if err := h.Store.UpsertCustomSkill(r.Context(), db.CustomSkill{ID: s.ID, YAML: req.YAML}); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	h.Skills.Upsert(s)

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionSkillUpdate
		e.TargetType = ptr(activity.TargetSkill)
		e.TargetID = ptr(s.ID)
		e.Detail = map[string]string{"id": s.ID, "name": s.Name}
	}
	writeJSON(w, http.StatusOK, detail(s))
}

// DeleteSkill removes a custom skill from the store and the live library.
// Operator+; audited. Built-in skills cannot be deleted (403).
func (h *Handlers) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	if h.Skills == nil {
		writeError(w, http.StatusServiceUnavailable, "skills_unavailable")
		return
	}
	id := chi.URLParam(r, "id")
	existing, ok := h.Skills.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if existing.Source != skills.SourceCustom {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "builtin_readonly", "detail": "built-in skills cannot be deleted"})
		return
	}
	if err := h.Store.DeleteCustomSkill(r.Context(), id); err != nil && !errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	h.Skills.Remove(id)

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionSkillDelete
		e.TargetType = ptr(activity.TargetSkill)
		e.TargetID = ptr(id)
		e.Detail = map[string]string{"id": id, "name": existing.Name}
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSkillRaw returns the YAML source of a skill so it can be opened in the
// editor: a custom skill returns its stored verbatim YAML; a built-in skill
// returns its YAML re-marshalled, which is handy for cloning one as a starting
// point for a new custom skill. Authenticated (reference data).
func (h *Handlers) GetSkillRaw(w http.ResponseWriter, r *http.Request) {
	if h.Skills == nil {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	id := chi.URLParam(r, "id")
	s, ok := h.Skills.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var yamlText string
	if s.Source == skills.SourceCustom {
		if cs, err := h.Store.GetCustomSkill(r.Context(), id); err == nil {
			yamlText = cs.YAML
		}
	}
	if yamlText == "" {
		b, err := skills.Marshal(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "marshal_failed")
			return
		}
		yamlText = string(b)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       s.ID,
		"yaml":     yamlText,
		"source":   s.Source,
		"editable": s.Source == skills.SourceCustom,
	})
}

type skillGenerateRequest struct {
	ContainerID string `json:"container_id"`
	Image       string `json:"image"`
	Notes       string `json:"notes"`
}

// GenerateSkill asks the configured AI provider to draft a skill YAML for a
// container (by id) or image. It does NOT persist anything — the draft is
// returned for the operator to review, edit, and then POST. Operator+; audited.
func (h *Handlers) GenerateSkill(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	var req skillGenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	image := strings.TrimSpace(req.Image)
	var name, compose string
	if req.ContainerID != "" {
		ctr, err := h.Store.GetContainer(r.Context(), req.ContainerID)
		if err != nil {
			writeError(w, http.StatusNotFound, "container_not_found")
			return
		}
		if image == "" {
			image = ctr.Image
		}
		name, compose = ctr.Name, ctr.ComposeProject
	}
	if image == "" {
		writeError(w, http.StatusBadRequest, "image_or_container_required")
		return
	}

	prompt := buildSkillGenPrompt(image, name, compose, req.Notes)

	ctx, cancel := context.WithTimeout(r.Context(), aiAskTimeout)
	defer cancel()
	resp, provider, err := h.AI.Ask(ctx, "generate_skill", prompt, "")
	if errors.Is(err, ai.ErrNotConfigured) {
		writeError(w, http.StatusBadRequest, "ai_not_configured")
		return
	} else if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("skill generate failed", "error", err)
		}
		var provErr *ai.ProviderError
		if errors.As(err, &provErr) {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "ai_request_failed", "detail": provErr.Error()})
		} else {
			writeError(w, http.StatusBadGateway, "ai_request_failed")
		}
		return
	}

	yamlText := extractYAML(resp.Answer)
	valid := true
	var parseErr string
	if _, perr := skills.Parse([]byte(yamlText)); perr != nil {
		valid = false
		parseErr = perr.Error()
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionSkillGenerate
		e.TargetType = ptr(activity.TargetSkill)
		e.Detail = map[string]string{
			"image":         image,
			"provider":      provider,
			"output_tokens": strconv.Itoa(resp.OutputTokens),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"yaml":        yamlText,
		"image":       image,
		"valid":       valid,
		"parse_error": parseErr,
		"provider":    provider,
	})
}

// buildSkillGenPrompt assembles the instruction that asks the model to emit a
// single skill YAML document for the given container image. The schema is spelled
// out so the output drops straight into the library.
func buildSkillGenPrompt(image, name, compose, notes string) string {
	var b strings.Builder
	b.WriteString("You are authoring a container-troubleshooting \"skill\" for a self-hosted infrastructure tool. ")
	b.WriteString("Output ONLY a single YAML document (no prose, no markdown fences) describing how to diagnose and fix the most common operational problems for this container.\n\n")
	b.WriteString("Target container:\n")
	b.WriteString("- image: " + image + "\n")
	if name != "" {
		b.WriteString("- container name: " + name + "\n")
	}
	if compose != "" {
		b.WriteString("- compose project: " + compose + "\n")
	}
	if strings.TrimSpace(notes) != "" {
		b.WriteString("- operator notes: " + strings.TrimSpace(notes) + "\n")
	}
	b.WriteString("\nUse EXACTLY this schema and field names:\n")
	b.WriteString(`id: <short-kebab-case-unique-id, e.g. derived from the image>
name: <human readable name>
version: "1.0"
category: <one of: media, network, database, monitoring, productivity, security, development, ai, automation, storage, communication, other>
description: <one sentence on what this container does>
docs_url: <official docs URL or "">
container_match:
  image_patterns:
    - <substring of the image ref that identifies it, e.g. the repo/name without tag>
  port_hints:
    - <typical published port number(s), integers>
common_issues:
  - id: <kebab-case>
    name: <short problem title>
    symptoms:
      - <observable symptom>
    trigger_conditions:
      - log_pattern: <substring/regex seen in logs when this happens>
    steps:
      - id: <kebab-case>
        description: <what this step checks or does, in plain language>
        type: <check|fix|inform>
        command: <a safe shell/docker command using {container_name} as a placeholder, or "">
        requires_approval: <true for any state-changing fix, false for read-only checks>
`)
	b.WriteString("\nProvide 2-4 realistic common_issues with concrete commands. ")
	b.WriteString("Mark every state-changing step requires_approval: true. Do not invent secrets or destructive commands. Output the YAML only.")
	return b.String()
}

// extractYAML pulls the YAML body out of a model answer: if the answer contains a
// fenced code block (```yaml ... ``` or ``` ... ```), the first block's contents
// are returned; otherwise the trimmed answer is returned as-is.
func extractYAML(answer string) string {
	s := strings.TrimSpace(answer)
	const fence = "```"
	start := strings.Index(s, fence)
	if start < 0 {
		return s
	}
	rest := s[start+len(fence):]
	// Drop an optional language tag on the opening fence line (e.g. "yaml").
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		firstLine := strings.TrimSpace(rest[:nl])
		if firstLine == "" || !strings.ContainsAny(firstLine, " \t") && len(firstLine) <= 12 {
			rest = rest[nl+1:]
		}
	}
	if end := strings.Index(rest, fence); end >= 0 {
		return strings.TrimSpace(rest[:end])
	}
	return strings.TrimSpace(rest)
}
