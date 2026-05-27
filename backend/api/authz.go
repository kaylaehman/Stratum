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
// "admin_only" error code that admin-only routes have always returned.
func (h *Handlers) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok || u.Role != auth.RoleAdmin {
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
