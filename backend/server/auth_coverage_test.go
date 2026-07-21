package server_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/api"
	"github.com/KAE-Labs/stratum/backend/server"
)

var routeParamRe = regexp.MustCompile(`\{[^}]+\}`)

// concretePath turns a chi route pattern into a requestable path by replacing
// every {param} (and a trailing wildcard) with a placeholder segment.
func concretePath(route string) string {
	p := routeParamRe.ReplaceAllString(route, "x")
	p = strings.ReplaceAll(p, "/*", "/x")
	return p
}

// TestAllRoutesRequireAuth is the structural enforcement of SECURITY.md's
// central claim: every API route rejects an unauthenticated request, EXCEPT the
// documented pre-auth bootstrap routes (/health, setup status/admin, auth
// login/refresh). A future route that forgets to mount under the auth middleware
// returns 200/403/etc. without a token and fails here — the cert bug proved that
// asserted controls drift silently, so this turns the claim into a test.
func TestAllRoutesRequireAuth(t *testing.T) {
	srv, _ := newNodeTestServer(t) // live server with real auth middleware

	// Enumerate the route table from an independently-built router (same routes).
	enumRouter := server.NewRouter(&server.Deps{Handlers: &api.Handlers{}})
	routes, ok := enumRouter.(chi.Routes)
	if !ok {
		t.Fatal("router does not implement chi.Routes")
	}

	// The ONLY routes allowed to answer without authentication. Anything else
	// answering a tokenless request with something other than 401 is a bug.
	public := map[string]bool{
		"GET /health":            true,
		"GET /metrics":           true, // only registered when a Prom registry is wired
		"GET /api/setup/status":  true, // pre-auth: "is an admin set up yet?"
		"POST /api/setup/admin":  true, // pre-auth: first-run admin creation
		"POST /api/auth/login":   true,
		"POST /api/auth/refresh": true, // uses the refresh cookie, not a bearer token
		// Agent enrollment: authenticated by a single-use enrollment token (checked
		// in-handler), not a JWT session — the install script runs on a fresh node
		// with no session. Rejects a tokenless request with 401/503 via its own gate.
		"GET /api/nodes/{id}/agent/binary":  true,
		"POST /api/nodes/{id}/agent/enroll": true,
	}

	c := &http.Client{}
	walkErr := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if public[key] {
			return nil
		}
		req, err := http.NewRequest(method, srv.URL+concretePath(route), nil)
		if err != nil {
			t.Errorf("%s: build request: %v", key, err)
			return nil
		}
		// Deliberately NO Authorization header.
		resp, err := c.Do(req)
		if err != nil {
			t.Errorf("%s: %v", key, err)
			return nil
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s answered %d without auth, want 401 — route is not behind the auth middleware (or add it to the public allowlist if intentional)", key, resp.StatusCode)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("chi.Walk: %v", walkErr)
	}
}
