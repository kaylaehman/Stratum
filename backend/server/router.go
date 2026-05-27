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

			// Read-only node + tree routes.
			r.Get("/nodes", d.Handlers.ListNodes)
			r.Get("/nodes/{id}", d.Handlers.GetNode)
			r.Get("/tree", d.Handlers.Tree)
			r.Get("/nodes/{id}/vms", d.Handlers.NodeVMs)
			r.Get("/nodes/{id}/containers", d.Handlers.NodeContainers)

			// UID/GID visualizer + container inspect (read-only).
			r.Get("/nodes/{id}/users", d.Handlers.HostUsers)
			r.Get("/containers/{id}", d.Handlers.InspectContainer)
			r.Get("/containers/{id}/users", d.Handlers.ContainerUsersHandler)
			r.Get("/containers/{id}/uid-analysis", d.Handlers.UIDAnalysis)
			r.Get("/containers/{id}/uid-analysis.csv", d.Handlers.UIDAnalysisCSV)
			r.Get("/containers/{id}/uid-analysis.json", d.Handlers.UIDAnalysisJSON)
			r.Get("/containers/{id}/file-uid", d.Handlers.FileUID)
			r.Post("/containers/{id}/diagnostic", d.Handlers.Diagnostic)

			// Log streaming subscription (server-side topic grant; lines flow over /api/ws).
			r.Post("/logs/subscribe", d.Handlers.LogsSubscribe)
			r.Post("/logs/unsubscribe", d.Handlers.LogsUnsubscribe)

			// Bind-mount tracer (read-only).
			r.Get("/containers/{id}/mounts", d.Handlers.ContainerMounts)
			r.Get("/nodes/{id}/mounts", d.Handlers.ReverseMounts)
			r.Get("/nodes/{id}/mounts/shared", d.Handlers.SharedMounts)

			// Activity log (admin gate enforced in handlers).
			r.Get("/activity", d.Handlers.ActivityList)
			r.Get("/activity/export.csv", d.Handlers.ActivityExportCSV)
			r.Get("/activity/actions", d.Handlers.ActivityActions)

			// Security: ports audit + privileged flag (admin gate enforced in handlers).
			r.Get("/security/ports", d.Handlers.Ports)
			r.Get("/security/privileged", d.Handlers.Privileged)
			r.Get("/containers/security-badges", d.Handlers.SecurityBadges)
			r.Get("/containers/{id}/security", d.Handlers.ContainerSecurity)

			// Volume health (read-only; cross-node).
			r.Get("/volumes", d.Handlers.ListVolumes)

			// Resource timeline (read-only).
			r.Get("/containers/{id}/metrics", d.Handlers.ContainerMetrics)
			r.Get("/containers/{id}/metrics.csv", d.Handlers.ContainerMetricsCSV)

			// Network topology (read-only; per node).
			r.Get("/nodes/{id}/topology", d.Handlers.NodeTopology)

			// Dependency graph (read-only; per node).
			r.Get("/nodes/{id}/depgraph", d.Handlers.NodeDependencyGraph)

			// Global search (read-only).
			r.Get("/search", d.Handlers.Search)

			// Wake-on-LAN config read (set/wake are audited mutations below).
			r.Get("/nodes/{id}/wol", d.Handlers.GetWOL)

			// Notification webhooks (list + test read-side; CRUD is audited below).
			r.Get("/webhooks", d.Handlers.ListWebhooks)
			r.Post("/webhooks/{id}/test", d.Handlers.TestWebhook)

			// Bookmarks (per-user prefs; not infra mutations, so not audited).
			r.Get("/bookmarks", d.Handlers.ListBookmarks)
			r.Post("/bookmarks", d.Handlers.CreateBookmark)
			r.Put("/bookmarks/reorder", d.Handlers.ReorderBookmarks)
			r.Delete("/bookmarks/{id}", d.Handlers.DeleteBookmark)

			// Filesystem reads (admin-gated writes are in the audited group).
			r.Get("/nodes/{id}/fs", d.Handlers.FSList)
			r.Get("/nodes/{id}/fs/file", d.Handlers.FSReadFile)
			r.Get("/nodes/{id}/fs/download", d.Handlers.FSDownload)

			// Authenticated + audited mutations.
			audited := r.With(mw.Activity(d.Handlers.Activity))
			audited.Post("/auth/logout", d.Handlers.Logout)
			audited.Post("/nodes", d.Handlers.CreateNode)
			audited.Put("/nodes/{id}", d.Handlers.RenameNode)
			audited.Delete("/nodes/{id}", d.Handlers.DeleteNode)
			audited.Post("/nodes/{id}/probe", d.Handlers.ReprobeNode)
			audited.Post("/nodes/probe-preview", d.Handlers.ProbePreview)
			audited.Put("/nodes/{id}/fs/file", d.Handlers.FSWriteFile)
			audited.Post("/nodes/{id}/fs/upload", d.Handlers.FSUpload)
			audited.Post("/nodes/{id}/fs/mkdir", d.Handlers.FSMkdir)
			audited.Post("/nodes/{id}/fs/rename", d.Handlers.FSRename)
			audited.Delete("/nodes/{id}/fs", d.Handlers.FSDelete)
			audited.Post("/security/acknowledge", d.Handlers.AcknowledgeFlag)
			audited.Delete("/security/acknowledge/{id}", d.Handlers.RevokeAcknowledgement)
			audited.Post("/security/rescan", d.Handlers.Rescan)
			audited.Delete("/nodes/{id}/volumes/{name}", d.Handlers.RemoveVolume)
			audited.Post("/containers/{id}/start", d.Handlers.StartContainer)
			audited.Post("/containers/{id}/stop", d.Handlers.StopContainer)
			audited.Post("/containers/{id}/restart", d.Handlers.RestartContainer)
			audited.Post("/containers/bulk", d.Handlers.BulkContainers)
			audited.Put("/nodes/{id}/wol", d.Handlers.SetWOL)
			audited.Post("/nodes/{id}/wake", d.Handlers.WakeNode)
			audited.Post("/webhooks", d.Handlers.CreateWebhook)
			audited.Put("/webhooks/{id}", d.Handlers.UpdateWebhook)
			audited.Delete("/webhooks/{id}", d.Handlers.DeleteWebhook)
		})
	})

	return r
}
