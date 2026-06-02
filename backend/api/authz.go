package api

import (
	"net/http"

	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/features"
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
// (Feature F7). It is FAIL-CLOSED: when the step-up feature is enabled, a caller
// must have TOTP enrolled AND a valid step-up grace window. The outcomes:
//
//   - feature.action_2fa disabled (or 2FA subsystem unwired) → allowed (no-op).
//   - no TOTP enrolled → 428 {error:"totp_enrollment_required"}; the client
//     directs the user to enroll. (Previously this returned true, silently
//     bypassing the documented control — see SECURITY.md "admin + TOTP step-up".)
//   - enrolled but no fresh confirmation → 428 {error:"2fa_required"}; the client
//     prompts for a code, POSTs /api/me/2fa/challenge, and retries.
//
// The gate is intentionally role-agnostic: every step-up-guarded action is
// high-risk regardless of who invokes it. Admins who want to opt out disable the
// feature flag.
func (h *Handlers) requireStepUp(w http.ResponseWriter, r *http.Request) bool {
	if h.TwoFA == nil {
		return true // 2FA subsystem not wired (e.g. minimal builds/tests)
	}
	// Respect the feature flag: when step-up 2FA is off, do not enforce. Without
	// this, disabling the flag would not actually disable enforcement.
	if h.Features != nil && !h.Features.Enabled(r.Context(), features.FlagActionStepUp) {
		return true
	}
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if !h.TwoFA.Enabled(r.Context(), u.ID) {
		// Fail closed: a high-risk action must not proceed for a user with no
		// second factor. They must enroll first.
		writeError(w, http.StatusPreconditionRequired, "totp_enrollment_required")
		return false
	}
	if !h.TwoFA.HasStepUp(u.ID) {
		writeError(w, http.StatusPreconditionRequired, "2fa_required")
		return false
	}
	return true
}
