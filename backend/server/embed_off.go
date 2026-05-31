//go:build !embed

// This file is compiled in the default (no-tag) build.  It provides a no-op
// mountSPA so router.go can call it unconditionally without any build-tag
// guards in the shared file.  The SPA is NOT served; the frontend runs
// separately (e.g. Vite dev server on :5173 proxied to the backend on :8080).
package server

import "net/http"

// mountSPA is a no-op in the default build.
func mountSPA(_ interface{ Handle(pattern string, handler http.Handler) }) {}
