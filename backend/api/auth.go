package api

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/middleware"
)

const (
	refreshCookieName = "stratum_refresh"
	refreshTTL        = 30 * 24 * time.Hour
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userView struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role"`
}

// Login verifies credentials, issues an access token, and sets a rotating
// refresh cookie scoped to /api/auth.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	user, err := h.Store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		h.logLogin(r, nil, activity.ResultError)
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if auth.CheckPassword(user.PasswordHash, req.Password) != nil {
		h.logLogin(r, &user.ID, activity.ResultError)
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	access, _, exp, err := h.JWT.Issue(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_failed")
		return
	}
	if err := h.newSessionCookie(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "session_failed")
		return
	}

	h.logLogin(r, &user.ID, activity.ResultSuccess)
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access,
		"expires_at":   exp.UTC().Format(time.RFC3339),
		"user":         userView{ID: user.ID, Username: user.Username, Email: user.Email, Role: user.Role},
	})
}

// Refresh validates the refresh cookie, rotates the session, and issues a new
// access token.
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	sessionID, raw, ok := parseRefreshCookie(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no_refresh")
		return
	}
	sess, err := h.Store.GetSession(r.Context(), sessionID)
	if err != nil || sess.RevokedAt != nil || time.Now().After(sess.ExpiresAt) {
		writeError(w, http.StatusUnauthorized, "invalid_refresh")
		return
	}
	if subtle.ConstantTimeCompare([]byte(auth.HashRefreshToken(raw)), []byte(sess.RefreshHash)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid_refresh")
		return
	}

	// Rotate: revoke the old session, mint a new one.
	_ = h.Store.RevokeSession(r.Context(), sessionID, time.Now())
	if err := h.newSessionCookie(w, r, sess.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "session_failed")
		return
	}
	access, _, exp, err := h.JWT.Issue(sess.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access,
		"expires_at":   exp.UTC().Format(time.RFC3339),
	})
}

// Logout revokes the current refresh session and clears the cookie.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if sessionID, _, ok := parseRefreshCookie(r); ok {
		_ = h.Store.RevokeSession(r.Context(), sessionID, time.Now())
	}
	clearRefreshCookie(w, h.SecureCookies)
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = "auth.logout"
	}
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the authenticated user.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, userView{ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role})
}

// --- helpers ---

func (h *Handlers) newSessionCookie(w http.ResponseWriter, r *http.Request, userID string) error {
	raw, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return err
	}
	sess := db.Session{
		ID:          uuid.NewString(),
		UserID:      userID,
		RefreshHash: hash,
		UserAgent:   r.UserAgent(),
		IP:          clientIP(r),
		ExpiresAt:   time.Now().Add(refreshTTL),
	}
	if err := h.Store.CreateSession(r.Context(), sess); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    sess.ID + ":" + raw,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteStrictMode,
		Expires:  sess.ExpiresAt,
	})
	return nil
}

func clearRefreshCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func parseRefreshCookie(r *http.Request) (sessionID, raw string, ok bool) {
	c, err := r.Cookie(refreshCookieName)
	if err != nil {
		return "", "", false
	}
	id, token, found := strings.Cut(c.Value, ":")
	if !found || id == "" || token == "" {
		return "", "", false
	}
	return id, token, true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handlers) logLogin(r *http.Request, userID *string, result string) {
	_ = h.Activity.Append(r.Context(), activity.Entry{
		UserID: userID,
		Action: "auth.login",
		Result: result,
	})
}
