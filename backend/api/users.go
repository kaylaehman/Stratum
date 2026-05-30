package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/middleware"
)

// userAdminView is the admin-facing user record. It never carries the password
// hash.
type userAdminView struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

func toUserAdminView(u db.User) userAdminView {
	return userAdminView{
		ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role,
		CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// ListUsers returns all users (admin only). Feature 30.
func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	users, err := h.Store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]userAdminView, 0, len(users))
	for _, u := range users {
		out = append(out, toUserAdminView(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// CreateUser provisions a teammate account with an assigned role (admin only).
// Email-invite flows are a follow-on; for now the admin sets an initial
// password out-of-band. Audited.
func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.Username == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "username_required_password_min_8")
		return
	}
	if !auth.RoleValid(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid_role")
		return
	}
	// Reject duplicate usernames with a clear 409 rather than a generic 500.
	if _, err := h.Store.GetUserByUsername(r.Context(), req.Username); err == nil {
		writeError(w, http.StatusConflict, "username_taken")
		return
	} else if !errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash_failed")
		return
	}
	u := db.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         req.Role,
	}
	if err := h.Store.CreateUser(r.Context(), u); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	auditUser(r, "user.create", u.ID, map[string]string{"username": u.Username, "role": u.Role})
	writeJSON(w, http.StatusCreated, toUserAdminView(u))
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

// UpdateUserRole changes a user's role (admin only). It refuses to demote the
// last remaining admin, which would lock everyone out of admin functions.
// Audited.
func (h *Handlers) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireRole(w, r, auth.RoleAdmin)
	if !ok {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var req updateRoleRequest
	if err := decodeJSON(r, &req); err != nil || !auth.RoleValid(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid_role")
		return
	}
	// Serialise with DeleteUser so a concurrent demote+delete can't both pass
	// the last-admin guard and zero out the admins.
	h.userMu.Lock()
	defer h.userMu.Unlock()
	target, err := h.Store.GetUserByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if req.Role == target.Role {
		writeJSON(w, http.StatusOK, toUserAdminView(target)) // no-op
		return
	}
	// Guard: don't strip admin from the last admin (covers demoting yourself too).
	if target.Role == auth.RoleAdmin && req.Role != auth.RoleAdmin {
		if !h.canLoseAnAdmin(r, w) {
			return
		}
	}
	if err := h.Store.UpdateUserRole(r.Context(), id, req.Role); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	auditUser(r, "user.role", id, map[string]string{
		"username": target.Username, "from": target.Role, "to": req.Role, "by": actor.Username,
	})
	target.Role = req.Role
	writeJSON(w, http.StatusOK, toUserAdminView(target))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangeOwnPassword lets the authenticated user change their own password after
// re-verifying the current one. Any user (any role) may change their own
// password. Audited.
func (h *Handlers) ChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "password_min_8")
		return
	}
	// Fetch the full record: the context user comes from JWT claims and carries
	// no password hash.
	full, err := h.Store.GetUserByID(r.Context(), actor.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if auth.CheckPassword(full.PasswordHash, req.CurrentPassword) != nil {
		writeError(w, http.StatusForbidden, "current_password_incorrect")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash_failed")
		return
	}
	if err := h.Store.UpdatePasswordHash(r.Context(), actor.ID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	auditUser(r, "auth.password_change", actor.ID, map[string]string{"username": actor.Username})
	w.WriteHeader(http.StatusNoContent)
}

type adminUpdateUserRequest struct {
	Username *string `json:"username,omitempty"`
	Email    *string `json:"email,omitempty"`
	Password *string `json:"password,omitempty"`
}

// UpdateUser lets an admin edit another user's username/email and reset their
// password (admin only + step-up). A username/email change keeps sessions; a
// password reset revokes the target's sessions so they must re-authenticate.
// Audited.
func (h *Handlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireRole(w, r, auth.RoleAdmin)
	if !ok {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var req adminUpdateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	// Serialise with role/delete (see userMu) so a concurrent rename can't race
	// the username-uniqueness check.
	h.userMu.Lock()
	defer h.userMu.Unlock()

	target, err := h.Store.GetUserByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	changes := map[string]string{}
	newUsername, newEmail := target.Username, target.Email

	if req.Username != nil && strings.TrimSpace(*req.Username) != target.Username {
		uname := strings.TrimSpace(*req.Username)
		if uname == "" {
			writeError(w, http.StatusBadRequest, "username_required")
			return
		}
		if _, err := h.Store.GetUserByUsername(r.Context(), uname); err == nil {
			writeError(w, http.StatusConflict, "username_taken")
			return
		} else if !errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		newUsername = uname
		changes["username"] = uname
	}
	if req.Email != nil && strings.TrimSpace(*req.Email) != target.Email {
		newEmail = strings.TrimSpace(*req.Email)
		changes["email"] = "updated"
	}
	if newUsername != target.Username || newEmail != target.Email {
		if err := h.Store.UpdateUserProfile(r.Context(), id, newUsername, newEmail); err != nil {
			writeError(w, http.StatusInternalServerError, "update_failed")
			return
		}
		target.Username, target.Email = newUsername, newEmail
	}

	if req.Password != nil {
		if len(*req.Password) < 8 {
			writeError(w, http.StatusBadRequest, "password_min_8")
			return
		}
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "hash_failed")
			return
		}
		if err := h.Store.UpdatePasswordHash(r.Context(), id, hash); err != nil {
			writeError(w, http.StatusInternalServerError, "update_failed")
			return
		}
		// Force re-auth everywhere for the target after a reset.
		if err := h.Store.RevokeAllUserSessions(r.Context(), id, time.Now()); err != nil && h.Logger != nil {
			h.Logger.Warn("revoke sessions after password reset failed", "user_id", id, "error", err)
		}
		changes["password"] = "reset"
	}

	if len(changes) == 0 {
		writeJSON(w, http.StatusOK, toUserAdminView(target)) // no-op
		return
	}
	changes["by"] = actor.Username
	auditUser(r, "user.update", id, changes)
	writeJSON(w, http.StatusOK, toUserAdminView(target))
}

// DeleteUser removes a user (admin only). It refuses to delete the last admin
// and refuses self-deletion (which would orphan the current session). The
// deleted user's sessions are revoked so refresh stops immediately. Audited.
func (h *Handlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireRole(w, r, auth.RoleAdmin)
	if !ok {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == actor.ID {
		writeError(w, http.StatusBadRequest, "cannot_delete_self")
		return
	}
	// Serialise with UpdateUserRole (see userMu) for the last-admin guard.
	h.userMu.Lock()
	defer h.userMu.Unlock()
	target, err := h.Store.GetUserByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if target.Role == auth.RoleAdmin {
		if !h.canLoseAnAdmin(r, w) {
			return
		}
	}
	if err := h.Store.DeleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	// The account is gone; failing to revoke its sessions leaves refresh tokens
	// live until expiry, so surface it (the delete itself already committed).
	if err := h.Store.RevokeAllUserSessions(r.Context(), id, time.Now()); err != nil && h.Logger != nil {
		h.Logger.Warn("revoke sessions after user delete failed", "user_id", id, "error", err)
	}
	auditUser(r, "user.delete", id, map[string]string{"username": target.Username, "by": actor.Username})
	w.WriteHeader(http.StatusNoContent)
}

// canLoseAnAdmin returns true when removing one admin still leaves at least one.
// On failure it writes a 409 and returns false.
func (h *Handlers) canLoseAnAdmin(r *http.Request, w http.ResponseWriter) bool {
	n, err := h.Store.CountUsersByRole(r.Context(), auth.RoleAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return false
	}
	if n <= 1 {
		writeError(w, http.StatusConflict, "last_admin")
		return false
	}
	return true
}

func auditUser(r *http.Request, action, targetID string, detail map[string]string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetUser)
		e.TargetID = &targetID
		e.Detail = detail
	}
}

// --- Session management (Feature 30) ---

type sessionView struct {
	ID        string `json:"id"`
	UserAgent string `json:"user_agent,omitempty"`
	IP        string `json:"ip,omitempty"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	Current   bool   `json:"current"`
	Active    bool   `json:"active"`
}

// ListSessions returns the calling user's own sessions (active + recent), with
// the session backing the current refresh cookie flagged.
func (h *Handlers) ListSessions(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessions, err := h.Store.ListSessionsByUser(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	currentID, _, _ := parseRefreshCookie(r)
	now := time.Now()
	out := make([]sessionView, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, sessionView{
			ID: s.ID, UserAgent: s.UserAgent, IP: s.IP,
			CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339),
			ExpiresAt: s.ExpiresAt.UTC().Format(time.RFC3339),
			Current:   s.ID == currentID,
			Active:    s.RevokedAt == nil && now.Before(s.ExpiresAt),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

// RevokeOwnSession revokes one of the caller's own sessions (e.g. "sign out
// other devices"). A user may only revoke sessions they own. Audited.
func (h *Handlers) RevokeOwnSession(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	sess, err := h.Store.GetSession(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) || (err == nil && sess.UserID != u.ID) {
		// Don't disclose whether another user's session id exists.
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := h.Store.RevokeSession(r.Context(), id, time.Now()); err != nil {
		writeError(w, http.StatusInternalServerError, "revoke_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = "auth.session_revoke"
		e.TargetType = ptr(activity.TargetUser)
		e.TargetID = &u.ID
		e.Detail = map[string]string{"session_id": id}
	}
	w.WriteHeader(http.StatusNoContent)
}

// PruneExpiredSessions hard-deletes all expired sessions belonging to the
// calling user. Useful for housekeeping after many tokens have naturally
// expired. Audited.
func (h *Handlers) PruneExpiredSessions(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	now := time.Now()
	if err := h.Store.DeleteExpiredSessionsByUser(r.Context(), u.ID, now); err != nil {
		writeError(w, http.StatusInternalServerError, "prune_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = "auth.sessions_prune_expired"
		e.TargetType = ptr(activity.TargetUser)
		e.TargetID = &u.ID
	}
	w.WriteHeader(http.StatusNoContent)
}
