package api

import (
	"net/http"

	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/middleware"
)

// requireRole gates a handler to a minimum role (Feature 30). On failure it
// writes 403 and returns ok=false; on success it returns the authenticated
// user. Enforcement is fail-closed: an unknown/empty role never qualifies.
func (h *Handlers) requireRole(w http.ResponseWriter, r *http.Request, min string) (db.User, bool) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok || !auth.AtLeast(u.Role, min) {
		writeError(w, http.StatusForbidden, "forbidden")
		return db.User{}, false
	}
	return u, true
}

// requireAdmin gates a route to the admin role. It keeps the historical
// "admin_only" error code that admin-only routes have always returned, but
// shares the fail-closed rank check with requireRole (single source of truth).
func (h *Handlers) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok || !auth.AtLeast(u.Role, auth.RoleAdmin) {
		writeError(w, http.StatusForbidden, "admin_only")
		return false
	}
	return true
}

// requireOperator gates a route to operator-or-higher (operators and admins).
// Used for non-destructive container lifecycle actions.
func (h *Handlers) requireOperator(w http.ResponseWriter, r *http.Request) bool {
	_, ok := h.requireRole(w, r, auth.RoleOperator)
	return ok
}

// requireStepUp enforces a recent TOTP confirmation before a high-risk action
// (Feature F7). When the user has 2FA enabled and lacks a valid step-up grace
// window, it writes 428 Precondition Required {error:"2fa_required"} and returns
// false; the client then prompts for a code, POSTs /api/me/2fa/challenge, and
// retries. Users WITHOUT 2FA enabled are not challenged (per spec, per-account
// 2FA is optional; global enforcement is a follow-on).
func (h *Handlers) requireStepUp(w http.ResponseWriter, r *http.Request) bool {
	if h.TwoFA == nil {
		return true
	}
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if !h.TwoFA.Enabled(r.Context(), u.ID) {
		return true // nothing to challenge
	}
	if !h.TwoFA.HasStepUp(u.ID) {
		writeError(w, http.StatusPreconditionRequired, "2fa_required")
		return false
	}
	return true
}
