//go:build embed

// When the "embed" build tag is set, mountSPA wires the embedded SPA handler
// (from backend/embedspa) as a chi catch-all route so the backend binary
// serves both the API and the React UI on one port.
package server

import (
	"net/http"

	"github.com/KAE-Labs/stratum/backend/embedspa"
)

// mountSPA installs the SPA catch-all for the embed build.
func mountSPA(mux interface{ Handle(pattern string, handler http.Handler) }) {
	if h := embedspa.Handler(); h != nil {
		mux.Handle("/*", h)
	}
}
