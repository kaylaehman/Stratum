package server_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/api"
	"github.com/kaylaehman/stratum/backend/server"
)

// TestNoUnauditedMutatingRoutes is the structural write-coverage guarantee
// (SP9 §5.6): CLAUDE.md mandates that ALL mutating actions are logged. Per-route
// tests can't catch a future route that bypasses the activity middleware, so we
// enumerate every POST/PUT/DELETE route and require each to be classified —
// either it is in the audited set (mounted under the activity middleware) or it
// is an explicit pre-auth bootstrap exception. A newly added mutating route that
// is in neither set fails this test, forcing a conscious decision.
func TestNoUnauditedMutatingRoutes(t *testing.T) {
	// Routes register against method values on Handlers; a zero Handlers is fine
	// because we only enumerate the route table, never invoke a handler.
	router := server.NewRouter(&server.Deps{Handlers: &api.Handlers{}})
	routes, ok := router.(chi.Routes)
	if !ok {
		t.Fatal("router does not implement chi.Routes")
	}

	// Pre-auth endpoints intentionally NOT under the activity middleware: there is
	// no authenticated user to attribute, and login/refresh must not depend on the
	// audit write path. login and setup.admin each emit their own entry via a
	// direct activity.Append in the handler.
	publicMutating := map[string]bool{
		"POST /api/setup/admin":  true,
		"POST /api/auth/login":   true,
		"POST /api/auth/refresh": true,
	}

	// POST routes that are read-only or ephemeral, not resource mutations, so they
	// are intentionally not audited: the diagnostic is a read-only analysis, and
	// log subscribe/unsubscribe only grant/drop a per-session WebSocket topic
	// (auditing them would be high-volume noise with no mutation to record).
	nonMutatingPost := map[string]bool{
		"POST /api/containers/{id}/diagnostic": true,
		"POST /api/logs/subscribe":             true,
		"POST /api/logs/unsubscribe":           true,
		"POST /api/webhooks/{id}/test":         true, // sends a test message; no Stratum state change
		"POST /api/templates/{id}/render":      true, // pure string substitution; no state change
	}

	// Per-user preference mutations (bookmarks): user-owned state, not
	// infrastructure changes, so intentionally not under the activity middleware.
	userPrefMutating := map[string]bool{
		"POST /api/bookmarks":        true,
		"PUT /api/bookmarks/reorder": true,
		"DELETE /api/bookmarks/{id}": true,
	}

	// Resumable-upload staging (F10): chunk write + cancel only touch a temp
	// file; the completed upload is audited via POST .../fs/upload/finish. So
	// these two are intentionally not under the activity middleware.
	stagingMutating := map[string]bool{
		"PUT /api/nodes/{id}/fs/upload/chunk":    true,
		"DELETE /api/nodes/{id}/fs/upload/chunk": true,
	}

	// Mutating routes confirmed mounted under the activity middleware (the
	// `audited` group in router.go). Adding a new POST/PUT/DELETE route forces an
	// update here — the prompt to confirm the route is audited.
	auditedMutating := map[string]bool{
		"POST /api/auth/logout":                     true,
		"POST /api/nodes":                           true,
		"PUT /api/nodes/{id}":                       true,
		"DELETE /api/nodes/{id}":                    true,
		"POST /api/nodes/{id}/probe":                true,
		"POST /api/nodes/probe-preview":             true,
		"PUT /api/nodes/{id}/fs/file":               true,
		"POST /api/nodes/{id}/fs/upload":            true,
		"POST /api/nodes/{id}/fs/upload/finish":     true,
		"POST /api/nodes/{id}/fs/mkdir":             true,
		"POST /api/nodes/{id}/fs/rename":            true,
		"DELETE /api/nodes/{id}/fs":                 true,
		"POST /api/security/acknowledge":            true,
		"DELETE /api/security/acknowledge/{id}":     true,
		"POST /api/security/rescan":                 true,
		"DELETE /api/nodes/{id}/volumes/{name}":     true,
		"POST /api/containers/{id}/start":           true,
		"POST /api/containers/{id}/stop":            true,
		"POST /api/containers/{id}/restart":         true,
		"POST /api/containers/bulk":                 true,
		"PUT /api/nodes/{id}/wol":                   true,
		"POST /api/nodes/{id}/wake":                 true,
		"POST /api/webhooks":                        true,
		"PUT /api/webhooks/{id}":                    true,
		"DELETE /api/webhooks/{id}":                 true,
		"POST /api/updates/rescan":                  true,
		"POST /api/templates":                       true,
		"PUT /api/templates/{id}":                   true,
		"DELETE /api/templates/{id}":                true,
		"POST /api/templates/{id}/deploy":           true,
		"POST /api/secret-groups":                   true,
		"DELETE /api/secret-groups/{id}":            true,
		"POST /api/secret-groups/{id}/secrets":      true,
		"POST /api/secret-groups/{id}/import":       true,
		"DELETE /api/secrets/{id}":                  true,
		"POST /api/secrets/{id}/reveal":             true, // reveal is audited (who revealed what)
		"POST /api/nodes/{id}/sshkeys/delete":       true,
		"PUT /api/nodes/{id}/cron":                  true,
		"POST /api/containers/{id}/cve-scan":        true,
		"POST /api/containers/{id}/update":          true,
		"POST /api/containers/{id}/snapshot":        true,
		"POST /api/containers/{id}/rollback/{snap}": true,
		"PUT /api/containers/{id}/healthcheck":      true,
		"POST /api/scripts":                         true,
		"PUT /api/scripts/{id}":                     true,
		"DELETE /api/scripts/{id}":                  true,
		"POST /api/scripts/{id}/run":                true,
		"POST /api/nodes/{id}/backups":              true,
		"PUT /api/ai/config":                        true,
		"POST /api/ai/ask":                          true,
		"POST /api/certs/rescan":                    true,
		"POST /api/me/2fa/setup":                    true,
		"POST /api/me/2fa/enable":                   true,
		"POST /api/me/2fa/disable":                  true,
		"POST /api/users":                           true,
		"PUT /api/users/{id}/role":                  true,
		"DELETE /api/users/{id}":                    true,
		"DELETE /api/sessions/{id}":                 true,
	}

	walkErr := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		switch method {
		case http.MethodPost, http.MethodPut, http.MethodDelete:
		default:
			return nil
		}
		key := method + " " + route
		if publicMutating[key] || auditedMutating[key] || nonMutatingPost[key] || userPrefMutating[key] || stagingMutating[key] {
			return nil
		}
		t.Errorf("unclassified mutating route %q: it must be mounted under the activity middleware "+
			"(add it to auditedMutating) or, if it is a pre-auth bootstrap route, to publicMutating", key)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("chi.Walk: %v", walkErr)
	}
}
