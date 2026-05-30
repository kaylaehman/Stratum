package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
)

type ctxKey string

const userCtxKey ctxKey = "stratum.user"

// UserStore is the subset of db.Store the auth middleware needs.
type UserStore interface {
	GetUserByID(ctx context.Context, id string) (db.User, error)
}

// Auth validates the bearer access token, loads the user, and injects it into
// the request context. It responds 401 on any failure.
func Auth(jwt *auth.JWT, store UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				writeUnauthorized(w)
				return
			}
			uid, err := jwt.Verify(token)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			user, err := store.GetUserByID(r.Context(), uid)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), userCtxKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext returns the authenticated user injected by Auth.
func UserFromContext(ctx context.Context) (db.User, bool) {
	u, ok := ctx.Value(userCtxKey).(db.User)
	return u, ok
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	// WebSocket fallback: a browser cannot set the Authorization header on a
	// WebSocket handshake, so the SPA passes the access token as the second
	// WebSocket subprotocol — new WebSocket(url, ["bearer", "<token>"]) — which
	// arrives here as "Sec-WebSocket-Protocol: bearer, <token>". JWT characters
	// (base64url + '.') are all valid subprotocol token characters.
	if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
		parts := strings.Split(proto, ",")
		if len(parts) >= 2 && strings.EqualFold(strings.TrimSpace(parts[0]), WSBearerSubprotocol) {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// WSBearerSubprotocol is the first WebSocket subprotocol the SPA offers; the
// second offered value carries the access token. The handler that upgrades the
// connection must echo this subprotocol back (AcceptOptions.Subprotocols) so the
// browser handshake completes.
const WSBearerSubprotocol = "bearer"

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}
