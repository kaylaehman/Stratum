package api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/agentinstall"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/middleware"
)

// maxCSRBytes bounds the enrollment request body.
const maxCSRBytes = 64 << 10

// agentBearer extracts a "Bearer <token>" credential from the Authorization
// header (the enrollment token used by the install script's node-side calls).
func agentBearer(r *http.Request) string {
	const p = "Bearer "
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, p) {
		return strings.TrimSpace(strings.TrimPrefix(h, p))
	}
	return ""
}

// agentTokenAllowed applies the per-IP rate limit shared by the token-authed
// agent endpoints.
func (h *Handlers) agentTokenAllowed(w http.ResponseWriter, r *http.Request) bool {
	if !h.AgentTokenLimiter.Allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return false
	}
	return true
}

// AgentInstallScript renders the hardened agent install script for a node.
// Admin + step-up gated and audited (it mints a single-use enrollment token —
// credential issuance).
func (h *Handlers) AgentInstallScript(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminStepUp(w, r) {
		return
	}
	if h.AgentInstall == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_install_not_configured")
		return
	}
	nodeID := chi.URLParam(r, "id")
	node, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	var createdBy string
	if u, ok := middleware.UserFromContext(r.Context()); ok {
		createdBy = u.ID
	}
	script, err := h.AgentInstall.RenderInstallScript(r.Context(), agentinstall.ScriptParams{
		NodeID: nodeID, Host: node.Host, CreatedBy: createdBy,
	})
	switch {
	case errors.Is(err, agentinstall.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "agent_install_not_configured")
		return
	case errors.Is(err, agentinstall.ErrBaseURLScheme):
		writeError(w, http.StatusBadRequest, "base_url_must_be_https")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = "agent.install"
		e.TargetType = ptr("node")
		e.TargetID = &nodeID
	}

	// JSON so the SPA (which must fetch this through apiFetch for the step-up 428
	// flow) can read the body and offer it as a download; the token embedded in
	// the script is single-use and short-lived.
	writeJSON(w, http.StatusOK, map[string]any{"script": script})
}

// AgentBinary streams the agent binary for the requested arch. Authenticated by
// the enrollment token (validated, NOT consumed — the binary is not a secret and
// the single use is reserved for enrollment). Node-scoped so the token binding
// is checked.
func (h *Handlers) AgentBinary(w http.ResponseWriter, r *http.Request) {
	if !h.agentTokenAllowed(w, r) {
		return
	}
	if h.AgentInstall == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_install_not_configured")
		return
	}
	nodeID := chi.URLParam(r, "id")
	ok, err := h.AgentInstall.ValidateToken(r.Context(), nodeID, agentBearer(r))
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid_enroll_token")
		return
	}
	data, err := h.AgentInstall.Binary(r.URL.Query().Get("arch"))
	if errors.Is(err, agentinstall.ErrBadArch) {
		writeError(w, http.StatusBadRequest, "unsupported_arch")
		return
	} else if err != nil {
		writeError(w, http.StatusServiceUnavailable, "agent_binary_unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(data)
}

// AgentEnroll signs the CSR the install script submits, consuming the single-use
// enrollment token. Returns the signed agent cert as PEM. Node-scoped.
func (h *Handlers) AgentEnroll(w http.ResponseWriter, r *http.Request) {
	if !h.agentTokenAllowed(w, r) {
		return
	}
	if h.AgentInstall == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_install_not_configured")
		return
	}
	nodeID := chi.URLParam(r, "id")
	csrPEM, err := io.ReadAll(io.LimitReader(r.Body, maxCSRBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	certPEM, err := h.AgentInstall.SignEnrollment(r.Context(), nodeID, agentBearer(r), csrPEM)
	switch {
	case errors.Is(err, agentinstall.ErrTokenInvalid):
		writeError(w, http.StatusUnauthorized, "invalid_enroll_token")
		return
	case errors.Is(err, agentinstall.ErrBadCSR):
		writeError(w, http.StatusBadRequest, "invalid_csr")
		return
	case errors.Is(err, agentinstall.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "agent_install_not_configured")
		return
	case err != nil:
		writeError(w, http.StatusBadRequest, "enroll_failed")
		return
	}

	if h.Activity != nil {
		_ = h.Activity.Append(r.Context(), activity.Entry{
			Action: "agent.enroll", TargetType: ptr("node"), TargetID: &nodeID,
			Result: activity.ResultSuccess,
		})
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	_, _ = w.Write(certPEM)
}
