// Package server wires the chi router and runs the HTTP server lifecycle.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/kaylaehman/stratum/backend/api"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	mw "github.com/kaylaehman/stratum/backend/middleware"
)

// Deps are everything the router needs to mount handlers and middleware.
type Deps struct {
	Handlers *api.Handlers
	JWT      *auth.JWT
	Store    db.Store
}

// NewRouter builds the chi router with the middleware order mandated by the
// foundation design (§5.2): RequestID → logging → Recoverer (global), then
// per-group auth and activity middleware.
func NewRouter(d *Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(mw.Logging(d.Handlers.Logger))
	r.Use(chimw.Recoverer)

	// Public, unauthenticated.
	r.Get("/health", d.Handlers.Health)

	r.Route("/api", func(r chi.Router) {
		r.Get("/setup/status", d.Handlers.SetupStatus)
		r.Post("/setup/admin", d.Handlers.SetupAdmin)
		r.Post("/auth/login", d.Handlers.Login)
		r.Post("/auth/refresh", d.Handlers.Refresh)

		// Authenticated.
		r.Group(func(r chi.Router) {
			r.Use(mw.Auth(d.JWT, d.Store))
			r.Get("/me", d.Handlers.Me)
			r.Get("/ws", d.Handlers.WebSocket)

			// Authenticated + audited mutations.
			r.With(mw.Activity(d.Handlers.Activity)).Post("/auth/logout", d.Handlers.Logout)
		})
	})

	return r
}
