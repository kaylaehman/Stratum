//go:build embed

// Package embedspa embeds the pre-built frontend SPA at compile time.
// The SPA assets must be copied into backend/embedspa/dist/ before building:
//
//	# From repo root:
//	cd frontend && npm run build
//	cp -r frontend/dist backend/embedspa/dist
//	cd backend && go build -tags embed -o stratum .
//
// Or with the Makefile/Taskfile target (see docs/single-binary.md).
// The split dev workflow (no embed tag) is unchanged.
package embedspa

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var spa embed.FS

// Handler returns an http.Handler that serves the embedded SPA.
// Paths not found in the FS fall through to index.html so React Router works.
func Handler() http.Handler {
	sub, err := fs.Sub(spa, "dist")
	if err != nil {
		panic("embedspa: could not sub dist: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve exact file if it exists.
		if f, err := sub.Open(r.URL.Path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// Fall back to index.html for client-side routing.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
