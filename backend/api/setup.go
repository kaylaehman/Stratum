package api

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
)

// SetupStatus reports whether first-run admin setup is still required.
func (h *Handlers) SetupStatus(w http.ResponseWriter, r *http.Request) {
	n, err := h.Store.CountUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_setup": n == 0})
}

type setupAdminRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// SetupAdmin creates the first admin user. It is a one-time, unauthenticated
// endpoint: once any user exists it returns 403 forever.
func (h *Handlers) SetupAdmin(w http.ResponseWriter, r *http.Request) {
	n, err := h.Store.CountUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if n > 0 {
		writeError(w, http.StatusForbidden, "setup_already_complete")
		return
	}

	var req setupAdminRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.Username == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "username_required_password_min_8")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash_failed")
		return
	}
	user := db.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         "admin",
	}
	if err := h.Store.CreateUser(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}

	_ = h.Activity.Append(r.Context(), activity.Entry{
		UserID:     &user.ID,
		Action:     "setup.admin",
		TargetType: ptr("user"),
		TargetID:   &user.ID,
		Result:     activity.ResultSuccess,
	})

	writeJSON(w, http.StatusCreated, map[string]string{"id": user.ID, "username": user.Username})
}
