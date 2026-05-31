package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/middleware"
	"github.com/kaylaehman/stratum/backend/remediation"
)

// ── Generate ─────────────────────────────────────────────────────────────────

type generateProposalBody struct {
	Source      string   `json:"source"`
	Title       string   `json:"title"`
	Rationale   string   `json:"rationale"`
	NodeID      string   `json:"node_id"`
	ContainerID string   `json:"container_id"`
	Commands    []string `json:"commands"`
}

// GenerateProposal creates a remediation proposal from a diagnostic result,
// runbook, or AI suggestion. RBAC: operator+. Never executes anything.
func (h *Handlers) GenerateProposal(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())

	var body generateProposalBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if body.Title == "" || body.NodeID == "" || len(body.Commands) == 0 {
		writeError(w, http.StatusBadRequest, "title_node_id_commands_required")
		return
	}

	p, err := h.Remediation.Generate(r.Context(), remediation.GenerateRequest{
		Source:      body.Source,
		Title:       body.Title,
		Rationale:   body.Rationale,
		NodeID:      body.NodeID,
		ContainerID: body.ContainerID,
		Commands:    body.Commands,
	}, user.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	auditRemediation(r, activity.ActionRemediationGenerated, p.ID, p.Title, p.RiskLevel)
	writeJSON(w, http.StatusCreated, p)
}

// ── List ─────────────────────────────────────────────────────────────────────

// ListProposals returns proposals, optionally filtered by node_id query param.
// RBAC: operator+.
func (h *Handlers) ListProposals(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	nodeID := r.URL.Query().Get("node_id")
	proposals, err := h.Remediation.List(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if proposals == nil {
		proposals = []db.RemediationProposal{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"proposals": proposals})
}

// ── Get ───────────────────────────────────────────────────────────────────────

// GetProposal returns a single proposal by id. RBAC: operator+.
func (h *Handlers) GetProposal(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	p, err := h.Remediation.Get(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ── Approve ───────────────────────────────────────────────────────────────────

// ApproveProposal transitions a proposal to approved. RBAC: admin for
// destructive proposals, operator+ for non-destructive.
func (h *Handlers) ApproveProposal(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	// Read the proposal first so we can gate destructive ones to admin.
	p, err := h.Remediation.Get(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	// Destructive proposals require admin.
	if p.RiskLevel == remediation.RiskDestructive {
		if !auth.AtLeast(user.Role, auth.RoleAdmin) {
			writeError(w, http.StatusForbidden, "admin_required_for_destructive")
			return
		}
	} else {
		if !auth.AtLeast(user.Role, auth.RoleOperator) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	updated, err := h.Remediation.Approve(r.Context(), id, user.ID)
	if errors.Is(err, remediation.ErrAlreadyTerminal) {
		writeError(w, http.StatusConflict, "already_terminal")
		return
	}
	if errors.Is(err, remediation.ErrInvalidTransition) {
		writeError(w, http.StatusConflict, "invalid_transition")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	auditRemediation(r, activity.ActionRemediationApproved, id, updated.Title, updated.RiskLevel)
	writeJSON(w, http.StatusOK, updated)
}

// ── Reject ────────────────────────────────────────────────────────────────────

// RejectProposal transitions a proposal to rejected. RBAC: operator+.
func (h *Handlers) RejectProposal(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	updated, err := h.Remediation.Reject(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if errors.Is(err, remediation.ErrAlreadyTerminal) {
		writeError(w, http.StatusConflict, "already_terminal")
		return
	}
	if errors.Is(err, remediation.ErrInvalidTransition) {
		writeError(w, http.StatusConflict, "invalid_transition")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	auditRemediation(r, activity.ActionRemediationRejected, id, updated.Title, updated.RiskLevel)
	writeJSON(w, http.StatusOK, updated)
}

// ── Execute ───────────────────────────────────────────────────────────────────

// ExecuteProposal runs the commands on the target node. Hard requirements:
//  1. Proposal must be in approved status (enforced by the service).
//  2. Destructive proposals require TOTP step-up (enforced by requireStepUp).
//  3. Admin only for destructive; operator+ otherwise.
//
// This is the ONLY path that runs commands on hosts — no auto-execution exists.
func (h *Handlers) ExecuteProposal(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")

	// Fetch the proposal to check risk before permitting execution.
	p, err := h.Remediation.Get(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	// Destructive proposals require admin AND TOTP step-up.
	if p.RiskLevel == remediation.RiskDestructive {
		if !auth.AtLeast(user.Role, auth.RoleAdmin) {
			writeError(w, http.StatusForbidden, "admin_required_for_destructive")
			return
		}
		if !h.requireStepUp(w, r) {
			return // 428 written by requireStepUp
		}
	} else {
		// Non-destructive: operator+ only.
		if !auth.AtLeast(user.Role, auth.RoleOperator) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	// Verify the target node has SSH capability (capability gate).
	node, err := h.Store.GetNode(r.Context(), p.NodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "node_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if !nodeCanExec(node) {
		writeError(w, http.StatusBadRequest, "node_cannot_exec")
		return
	}

	result, err := h.Remediation.Execute(r.Context(), id)
	if errors.Is(err, remediation.ErrNotApproved) {
		writeError(w, http.StatusConflict, "not_approved")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	action := activity.ActionRemediationExecuted
	if result.Status == remediation.StatusFailed {
		action = activity.ActionRemediationFailed
	}
	auditRemediationExec(r, action, id, result.Title, result.RiskLevel, result.Stdout, result.Stderr)
	writeJSON(w, http.StatusOK, result)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// nodeCanExec returns true when the node's capabilities permit SSH/agent exec.
// Any registered node can be SSH-exec'd; Proxmox-API-only nodes cannot.
func nodeCanExec(node db.Node) bool {
	// If the node has no host it was never registered properly.
	return node.Host != ""
}

func auditRemediation(r *http.Request, action, id, title, risk string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetRemediation)
		e.TargetID = &id
		e.Detail = map[string]string{"title": title, "risk_level": risk}
	}
}

func auditRemediationExec(r *http.Request, action, id, title, risk, stdout, stderr string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetRemediation)
		e.TargetID = &id
		e.Detail = map[string]string{
			"title":      title,
			"risk_level": risk,
			"stdout":     truncate(stdout, 500),
			"stderr":     truncate(stderr, 500),
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
