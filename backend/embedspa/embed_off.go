//go:build !embed

// Package embedspa is a no-op in the default (no-tag) build.
// The Handler function returns nil so router.go can call it unconditionally.
package embedspa

import "net/http"

// Handler returns nil in the default build — the SPA is served separately
// (e.g. Vite dev server on :5173 or a dedicated nginx container).
func Handler() http.Handler { return nil }
