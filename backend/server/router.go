// Package server wires the chi router and runs the HTTP server lifecycle.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kaylaehman/stratum/backend/api"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/db"
	mw "github.com/kaylaehman/stratum/backend/middleware"
)

// Deps are everything the router needs to mount handlers and middleware.
type Deps struct {
	Handlers  *api.Handlers
	JWT       *auth.JWT
	Store     db.Store
	// PromRegistry is the Prometheus registry for Stratum's own metrics.
	// When non-nil, GET /metrics is registered as a Prometheus scrape endpoint,
	// guarded by MetricsToken (bearer token when set, else loopback-only).
	// When nil the route is omitted.
	PromRegistry *prometheus.Registry
	// MetricsToken, when set, is the bearer token required to scrape /metrics.
	// Empty means loopback-only access. See metricsGuard.
	MetricsToken string
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

	// Prometheus scrape endpoint — top-level, outside /api. Guarded by
	// metricsGuard (bearer token when STRATUM_METRICS_TOKEN is set, else
	// loopback-only). Mount only when a registry is provided; omitting keeps the
	// route absent in test setups that do not configure Prometheus.
	if d.PromRegistry != nil {
		metricsHandler := promhttp.HandlerFor(d.PromRegistry, promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		})
		r.Get("/metrics", metricsGuard(d.MetricsToken, metricsHandler).ServeHTTP)
	}

	r.Route("/api", func(r chi.Router) {
		r.Get("/setup/status", d.Handlers.SetupStatus)
		r.Post("/setup/admin", d.Handlers.SetupAdmin)
		r.Post("/auth/login", d.Handlers.Login)
		r.Post("/auth/refresh", d.Handlers.Refresh)

		// Authenticated.
		r.Group(func(r chi.Router) {
			r.Use(mw.Auth(d.JWT, d.Store))
			r.Get("/me", d.Handlers.Me)
			// Generic step-up gate check; the frontend calls this before opening a
			// step-up-gated WebSocket (e.g. the node terminal) so a 428 can surface
			// the StepUp modal that a WS handshake can't. Unaudited (no side effect).
			r.Post("/stepup/preflight", d.Handlers.StepUpPreflight)
			r.Get("/me/2fa", d.Handlers.TwoFAStatus)
			r.Get("/ws", d.Handlers.WebSocket)
			// Interactive host shell over SSH (admin-gated in handler; audited).
			r.Get("/nodes/{id}/terminal", d.Handlers.NodeTerminal)

			// User management (admin gate enforced in handlers) + own sessions.
			r.Get("/users", d.Handlers.ListUsers)
			r.Get("/sessions", d.Handlers.ListSessions)

			// Read-only node + tree routes.
			r.Get("/nodes", d.Handlers.ListNodes)
			r.Get("/nodes/{id}", d.Handlers.GetNode)
			r.Get("/tree", d.Handlers.Tree)
			r.Get("/nodes/{id}/vms", d.Handlers.NodeVMs)
			r.Get("/nodes/{id}/containers", d.Handlers.NodeContainers)

			// UID/GID visualizer + container inspect (read-only).
			r.Get("/nodes/{id}/users", d.Handlers.HostUsers)
			r.Get("/containers/{id}", d.Handlers.InspectContainer)
			r.Get("/containers/{id}/fs", d.Handlers.ContainerFSList)
			r.Get("/containers/{id}/fs/file", d.Handlers.ContainerFSFile)
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

			// Security posture score (admin-gated; read-only composition of stored data).
			r.Get("/nodes/{id}/posture", d.Handlers.NodePosture)

			// Image CVE scans (admin-gated; on-demand scan is audited below).
			r.Get("/security/cve", d.Handlers.CVEScans)
			r.Get("/security/cve/status", d.Handlers.CVEStatus)
			r.Get("/security/cve/{digest}", d.Handlers.CVEDetail)

			// Volume health (read-only; cross-node).
			r.Get("/volumes", d.Handlers.ListVolumes)

			// Resource timeline (read-only).
			r.Get("/containers/{id}/metrics", d.Handlers.ContainerMetrics)
			r.Get("/containers/{id}/metrics.csv", d.Handlers.ContainerMetricsCSV)

			// Container healthcheck config + history (read-only).
			r.Get("/containers/{id}/health", d.Handlers.ContainerHealth)

			// Rollback snapshots list (read-only; update/snapshot/rollback audited below).
			r.Get("/containers/{id}/snapshots", d.Handlers.ListSnapshots)

			// Per-container reverse-proxy view (admin gate in handler).
			r.Get("/containers/{id}/proxy", d.Handlers.ContainerProxy)

			// Network topology (read-only; per node).
			r.Get("/nodes/{id}/topology", d.Handlers.NodeTopology)

			// Dependency graph (read-only; per node).
			r.Get("/nodes/{id}/depgraph", d.Handlers.NodeDependencyGraph)

			// Global search (read-only).
			r.Get("/search", d.Handlers.Search)

			// Container-troubleshooting skill library (read reference data;
			// user-authored skills are created/edited via the audited group below).
			r.Get("/skills", d.Handlers.ListSkills)
			r.Get("/skills/{id}", d.Handlers.GetSkill)
			r.Get("/skills/{id}/raw", d.Handlers.GetSkillRaw)

			// AI assistant config (admin gate in handler; set/ask are audited below).
			r.Get("/ai/config", d.Handlers.AIConfigGet)
			r.Get("/ai/ollama/models", d.Handlers.AIGetOllamaModels)
			r.Get("/ai/oauth/start", d.Handlers.AIOAuthStart)

			// Certificate inventory (admin gate in handler; rescan is audited below).
			r.Get("/certs", d.Handlers.CertList)

			// AI agent memory (read; create/update/delete audited below).
			r.Get("/memory", d.Handlers.ListMemory)

			// AI runbooks (read; create/update/delete audited below).
			r.Get("/runbooks", d.Handlers.ListRunbooks)
			r.Get("/runbooks/{id}", d.Handlers.GetRunbook)
			r.Post("/runbooks/{id}/validate", d.Handlers.ValidateRunbook)

			// Agentic remediation (read; mutations audited below).
			r.Get("/remediation", d.Handlers.ListProposals)
			r.Get("/remediation/{id}", d.Handlers.GetProposal)

			// Reverse proxy detection + rules (admin gate in handler; config audited below).
			r.Get("/nodes/{id}/proxy", d.Handlers.NodeProxy)
			// Cloudflare account/tunnel discovery for the cloudflare-api provider
			// setup picker (admin gate in handler; read-only, token used in-memory).
			r.Post("/nodes/{id}/proxy/cloudflare/discover", d.Handlers.DiscoverProxyCloudflare)

			// DNS detection + records (admin gate in handler; config audited below).
			r.Get("/nodes/{id}/dns", d.Handlers.NodeDNS)

			// Feature flags (read; toggle audited below).
			r.Get("/features", d.Handlers.ListFeatures)

			// Chat bot config (admin gate in handler; set audited below).
			r.Get("/chat/config", d.Handlers.ChatConfigGet)

			// File change detection (admin gate in handler; mutations audited below).
			r.Get("/nodes/{id}/watches", d.Handlers.ListWatches)
			r.Get("/fileevents", d.Handlers.FileEvents)

			// SSO passthrough config (admin gate in handler; mutations audited below).
			r.Get("/sso", d.Handlers.ListSSO)

			// Wake-on-LAN config read (set/wake are audited mutations below).
			r.Get("/nodes/{id}/wol", d.Handlers.GetWOL)

			// Notification webhooks (list + test read-side; CRUD is audited below).
			r.Get("/webhooks", d.Handlers.ListWebhooks)
			r.Post("/webhooks/{id}/test", d.Handlers.TestWebhook)

			// Incident timeline (read-only; merges activity, containers, metrics, file events).
			r.Get("/incidents/timeline", d.Handlers.IncidentTimeline)

			// Uptime monitors (read; CRUD is audited below).
			r.Get("/uptime/monitors", d.Handlers.ListUptimeMonitors)
			r.Get("/uptime/monitors/{id}", d.Handlers.GetUptimeMonitor)
			r.Get("/uptime/monitors/{id}/history", d.Handlers.UptimeMonitorHistory)

			// Automations engine (read; mutations are audited below).
			r.Get("/automations", d.Handlers.ListAutomations)

			// Image update detection (read-only; cross-node).
			r.Get("/updates", d.Handlers.Updates)

			// Live stack: compose read + env-var list (docker-gated; degrades on SSH-only).
			r.Get("/nodes/{id}/stacks/{project}/compose", d.Handlers.GetStackCompose)
			r.Get("/nodes/{id}/stacks/{project}/env", d.Handlers.ListStackEnvVars)

			// Template library (read + render; CRUD/deploy are audited below).
			r.Get("/templates", d.Handlers.ListTemplates)
			r.Get("/templates/{id}", d.Handlers.GetTemplate)
			r.Post("/templates/{id}/render", d.Handlers.RenderTemplate)

			// Secrets vault list (key names only; mutations + reveal audited below).
			r.Get("/secrets", d.Handlers.ListSecrets)

			// SSH key audit (admin-gated in handler; delete is audited below).
			r.Get("/nodes/{id}/sshkeys", d.Handlers.ListSSHKeys)

			// Scheduled tasks: cron + systemd timers (admin-gated; cron edit below).
			r.Get("/nodes/{id}/schedule", d.Handlers.NodeSchedule)

			// Script library list (CRUD/run are audited below).
			r.Get("/scripts", d.Handlers.ListScripts)

			// Backup history + verify results (admin-gated; start/restore/verify are audited below).
			r.Get("/backups", d.Handlers.ListBackups)
			r.Get("/nodes/{id}/backups/verify", d.Handlers.ListBackupVerifyResults)

			// Config versions + drift (admin-gated; snapshot/revert are audited below).
			r.Get("/nodes/{id}/configversions", d.Handlers.ConfigVersionHistory)
			r.Get("/nodes/{id}/configversions/drift", d.Handlers.ConfigVersionDrift)

			// Capacity forecast (admin-gated; read-only).
			r.Get("/nodes/{id}/forecast", d.Handlers.NodeForecast)

			// Secret expiry list + node scan (admin-gated; set-expiry is audited below).
			r.Get("/secrets/expiring", d.Handlers.ListExpiringSecrets)
			r.Get("/secrets/scan", d.Handlers.ScanNodeSecrets)

			// Alert policies + delivery log (admin-gated; CRUD is audited below).
			r.Get("/alert-policies", d.Handlers.ListAlertPolicies)
			r.Get("/alert-deliveries", d.Handlers.ListAlertDeliveries)

			// DR export (admin-gated; format via ?format=json|yaml|md).
			r.Get("/dr-export", d.Handlers.DRExport)

			// Placement recommender (admin-gated; read-only; no audit).
			r.Get("/placement/recommend", d.Handlers.RecommendPlacement)

			// Web Push: VAPID key read (no auth role requirement; no audit).
			r.Get("/push/vapid-key", d.Handlers.VAPIDKey)

			// Bookmarks (per-user prefs; not infra mutations, so not audited).
			r.Get("/bookmarks", d.Handlers.ListBookmarks)
			r.Post("/bookmarks", d.Handlers.CreateBookmark)
			r.Put("/bookmarks/reorder", d.Handlers.ReorderBookmarks)
			r.Delete("/bookmarks/{id}", d.Handlers.DeleteBookmark)

			// Filesystem reads (admin-gated writes are in the audited group).
			r.Get("/nodes/{id}/fs", d.Handlers.FSList)
			r.Get("/nodes/{id}/fs/search", d.Handlers.FSSearch)
			r.Get("/nodes/{id}/fs/file", d.Handlers.FSReadFile)
			r.Get("/nodes/{id}/fs/download", d.Handlers.FSDownload)

			// Resumable upload staging (F10): status read + chunk/cancel write to a
			// temp file only. The completed upload (finish) is the audited event, so
			// these staging ops are intentionally not under the activity middleware.
			r.Get("/nodes/{id}/fs/upload/status", d.Handlers.FSUploadStatus)
			r.Put("/nodes/{id}/fs/upload/chunk", d.Handlers.FSUploadChunk)
			r.Delete("/nodes/{id}/fs/upload/chunk", d.Handlers.FSUploadCancel)

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
			audited.Post("/nodes/{id}/fs/upload/finish", d.Handlers.FSUploadFinish)
			audited.Post("/nodes/{id}/fs/mkdir", d.Handlers.FSMkdir)
			audited.Post("/nodes/{id}/fs/rename", d.Handlers.FSRename)
			audited.Delete("/nodes/{id}/fs", d.Handlers.FSDelete)
			audited.Post("/security/acknowledge", d.Handlers.AcknowledgeFlag)
			audited.Delete("/security/acknowledge/{id}", d.Handlers.RevokeAcknowledgement)
			audited.Post("/security/rescan", d.Handlers.Rescan)
			audited.Delete("/nodes/{id}/volumes/{name}", d.Handlers.RemoveVolume)
			audited.Post("/volumes/prune-unused", d.Handlers.PruneUnusedVolumes)
			audited.Post("/containers/{id}/start", d.Handlers.StartContainer)
			audited.Post("/containers/{id}/stop", d.Handlers.StopContainer)
			audited.Post("/containers/{id}/restart", d.Handlers.RestartContainer)
			audited.Post("/nodes/{id}/vms/{vmid}/{action}", d.Handlers.VMPowerAction)
			audited.Post("/nodes/{id}/power/{action}", d.Handlers.NodePowerAction)
			audited.Post("/containers/bulk", d.Handlers.BulkContainers)
			audited.Put("/nodes/{id}/wol", d.Handlers.SetWOL)
			audited.Post("/nodes/{id}/wake", d.Handlers.WakeNode)
			audited.Post("/webhooks", d.Handlers.CreateWebhook)
			audited.Put("/webhooks/{id}", d.Handlers.UpdateWebhook)
			audited.Delete("/webhooks/{id}", d.Handlers.DeleteWebhook)
			audited.Post("/updates/rescan", d.Handlers.RescanUpdates)
			// Stack edit + redeploy (audited; admin-gated in handler).
			audited.Post("/nodes/{id}/stacks/{project}/deploy", d.Handlers.RedeployStack)
			audited.Post("/nodes/{id}/stacks/{project}/lifecycle", d.Handlers.StackLifecycle)
			audited.Put("/nodes/{id}/stacks/{project}/env/{key}", d.Handlers.SetStackEnvVar)
			audited.Delete("/nodes/{id}/stacks/{project}/env/{key}", d.Handlers.DeleteStackEnvVar)
			audited.Post("/templates", d.Handlers.CreateTemplate)
			audited.Put("/templates/{id}", d.Handlers.UpdateTemplate)
			audited.Delete("/templates/{id}", d.Handlers.DeleteTemplate)
			audited.Post("/templates/{id}/deploy", d.Handlers.DeployTemplate)
			audited.Post("/secret-groups", d.Handlers.CreateSecretGroup)
			audited.Delete("/secret-groups/{id}", d.Handlers.DeleteSecretGroup)
			audited.Post("/secret-groups/{id}/secrets", d.Handlers.SetSecret)
			audited.Post("/secret-groups/{id}/import", d.Handlers.ImportSecrets)
			audited.Delete("/secrets/{id}", d.Handlers.DeleteSecret)
			audited.Post("/secrets/{id}/reveal", d.Handlers.RevealSecret)
			audited.Post("/nodes/{id}/sshkeys/delete", d.Handlers.DeleteSSHKey)
			audited.Put("/nodes/{id}/cron", d.Handlers.SetCron)
			audited.Post("/containers/{id}/cve-scan", d.Handlers.CVEScanContainer)
			audited.Post("/security/cve/bulk-scan", d.Handlers.CVEBulkScan)
			audited.Post("/containers/{id}/update", d.Handlers.UpdateContainer)
			audited.Post("/containers/{id}/snapshot", d.Handlers.SnapshotContainer)
			audited.Post("/containers/{id}/rollback/{snap}", d.Handlers.RollbackContainer)
			audited.Put("/containers/{id}/healthcheck", d.Handlers.SetHealthcheck)
			audited.Post("/containers/{id}/proxy", d.Handlers.AddContainerProxy)
			audited.Post("/scripts", d.Handlers.CreateScript)
			audited.Put("/scripts/{id}", d.Handlers.UpdateScript)
			audited.Delete("/scripts/{id}", d.Handlers.DeleteScript)
			audited.Post("/scripts/{id}/run", d.Handlers.RunScript)
			audited.Post("/nodes/{id}/backups", d.Handlers.StartBackup)
			audited.Post("/nodes/{id}/vms/{vmid}/backup", d.Handlers.StartGuestBackup)
			audited.Put("/ai/config", d.Handlers.AIConfigSet)
			audited.Post("/ai/ask", d.Handlers.AIAsk)
			audited.Post("/ai/oauth/exchange", d.Handlers.AIOAuthExchange)
			audited.Post("/ai/oauth/token", d.Handlers.AIOAuthToken)
			audited.Post("/ai/oauth/disconnect", d.Handlers.AIOAuthDisconnect)
			audited.Post("/certs/rescan", d.Handlers.CertRescan)
			audited.Put("/nodes/{id}/proxy/config", d.Handlers.SetNodeProxyConfig)
			audited.Put("/nodes/{id}/dns/config", d.Handlers.SetNodeDNSConfig)
			audited.Put("/features/{key}", d.Handlers.SetFeature)
			audited.Put("/chat/config", d.Handlers.ChatConfigSet)
			audited.Post("/nodes/{id}/watches", d.Handlers.AddWatch)
			audited.Delete("/nodes/{id}/watches/{watchID}", d.Handlers.RemoveWatch)
			audited.Post("/nodes/{id}/watches/scan", d.Handlers.ScanWatches)
			audited.Put("/sso", d.Handlers.UpsertSSO)
			audited.Delete("/sso/{id}", d.Handlers.DeleteSSO)
			audited.Post("/memory", d.Handlers.CreateMemory)
			audited.Put("/memory/{id}", d.Handlers.UpdateMemory)
			audited.Delete("/memory/{id}", d.Handlers.DeleteMemory)
			audited.Post("/runbooks", d.Handlers.CreateRunbook)
			audited.Put("/runbooks/{id}", d.Handlers.UpdateRunbook)
			audited.Delete("/runbooks/{id}", d.Handlers.DeleteRunbook)

			// Agentic remediation: generate / approve / reject / execute (all audited).
			audited.Post("/remediation", d.Handlers.GenerateProposal)
			audited.Post("/remediation/{id}/approve", d.Handlers.ApproveProposal)
			audited.Post("/remediation/{id}/reject", d.Handlers.RejectProposal)
			audited.Post("/remediation/{id}/execute", d.Handlers.ExecuteProposal)
			audited.Post("/skills", d.Handlers.CreateSkill)
			audited.Post("/skills/generate", d.Handlers.GenerateSkill)
			audited.Put("/skills/{id}", d.Handlers.UpdateSkill)
			audited.Delete("/skills/{id}", d.Handlers.DeleteSkill)
			audited.Post("/me/2fa/setup", d.Handlers.TwoFASetup)
			audited.Post("/me/2fa/enable", d.Handlers.TwoFAEnable)
			audited.Post("/me/2fa/disable", d.Handlers.TwoFADisable)
			audited.Post("/me/2fa/challenge", d.Handlers.TwoFAChallenge)
			audited.Post("/users", d.Handlers.CreateUser)
			audited.Put("/users/{id}/role", d.Handlers.UpdateUserRole)
			audited.Put("/users/{id}", d.Handlers.UpdateUser)
			audited.Delete("/users/{id}", d.Handlers.DeleteUser)
			audited.Delete("/sessions/{id}", d.Handlers.RevokeOwnSession)
			audited.Delete("/sessions/expired", d.Handlers.PruneExpiredSessions)
			audited.Post("/auth/change-password", d.Handlers.ChangeOwnPassword)

			// Uptime monitor mutations (audited).
			audited.Post("/uptime/monitors", d.Handlers.CreateUptimeMonitor)
			audited.Put("/uptime/monitors/{id}", d.Handlers.UpdateUptimeMonitor)
			audited.Delete("/uptime/monitors/{id}", d.Handlers.DeleteUptimeMonitor)

			// Automation mutations (audited).
			audited.Put("/automations/{key}", d.Handlers.UpdateAutomation)
			audited.Post("/automations/{key}/run", d.Handlers.RunAutomation)

			// Backup restore + verify (audited; destructive ops require step-up in handler).
			audited.Post("/nodes/{id}/backups/restore", d.Handlers.RestoreVolumeBackup)
			audited.Post("/nodes/{id}/backups/restore-guest", d.Handlers.RestoreGuestBackup)
			audited.Post("/nodes/{id}/backups/verify", d.Handlers.VerifyBackup)

			// Orchestration (audited).
			audited.Post("/orchestration/plan", d.Handlers.OrchestrationPlan)
			audited.Post("/orchestration/execute", d.Handlers.OrchestrationExecute)
			audited.Post("/nodes/{id}/drain", d.Handlers.NodeDrain)

			// Config versioning (audited).
			audited.Post("/nodes/{id}/configversions/snapshot", d.Handlers.ConfigVersionSnapshot)
			audited.Post("/nodes/{id}/configversions/revert", d.Handlers.ConfigVersionRevert)

			// Secret expiry (audited).
			audited.Put("/secrets/{id}/expiry", d.Handlers.SetSecretExpiry)

			// Alert policy CRUD (audited).
			audited.Post("/alert-policies", d.Handlers.CreateAlertPolicy)
			audited.Put("/alert-policies/{id}", d.Handlers.UpdateAlertPolicy)
			audited.Delete("/alert-policies/{id}", d.Handlers.DeleteAlertPolicy)

			// New compose stack (A9, audited).
			audited.Post("/nodes/{id}/stacks", d.Handlers.CreateStack)

			// Web Push mutations (C9, audited).
			audited.Post("/push/subscribe", d.Handlers.PushSubscribe)
			audited.Post("/push/unsubscribe", d.Handlers.PushUnsubscribe)
			audited.Post("/push/test", d.Handlers.PushTest)
		})
	})

	// Embed fallback: serve the SPA for all non-API routes.
	// In the default (no-tag) build, mountSPA is a no-op (embed_off.go).
	// With -tags embed it serves frontend/dist (embed_on.go).
	mountSPA(r)

	return r
}
