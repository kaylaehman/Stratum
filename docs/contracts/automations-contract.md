# Automations engine — contract (Wave 6)

A dedicated `/automations` page exposing 15 independently-configurable autonomous
automations on top of the existing services + remediation engine. Pattern mirrors
`features` (code-defined catalog + DB overrides) and the background-loop services
(`uptimeSvc.Run(ctx)` started in `main.go`).

> The single source of truth for the count and keys is the code catalog in
> `backend/automation/catalog.go` (`automation.Catalog()`); `TestCatalogHandlerParity`
> keeps it in lock-step with the executable handlers. This table started at 8 and
> grew to 15 — update it from the catalog, never hand-count.

## The 15 automations (code-defined catalog; stable keys)
| key | category | label | wires to | default |
|---|---|---|---|---|
| `restart_unhealthy` | self_heal | Restart unhealthy containers | container restart lifecycle | off |
| `auto_remediate_low` | self_heal | Auto-run low-risk remediation | remediation.Generate→Execute, ONLY when `ClassifyRisk`==low (never high/destructive) | off |
| `restart_on_resource_spike` | self_heal | Restart on resource spike | container restart on sustained CPU/RAM spike | off |
| `fix_bind_mount_perms` | self_heal | Fix bind-mount permissions | agent/SSH permission repair on bind mounts | off |
| `run_runbooks_on_alert` | self_heal | Run runbooks on alert | remediation runbooks triggered by alerts | off |
| `auto_pull_updates` | update | Auto-pull latest images | updates service (pull only, NO recreate) | off |
| `auto_update_containers` | update | Auto-update containers | recreate/update (pull+recreate); per-project allowlist in config | off |
| `scheduled_cve_scan` | security | Scheduled CVE scan | cve bulk scan over running images | off |
| `security_alerts` | security | Security change alerts | notifications/webhooks on new Critical CVE / new exposed port / new privileged / new SSH key | off |
| `patch_critical_cves` | security | Patch critical CVEs | recreate to a fixed image when a Critical CVE has a fix | off |
| `prune_unused_volumes` | maintenance | Prune unused volumes | volumes.PruneUnused | off |
| `scheduled_backups` | maintenance | Scheduled backups | backups.StartBackup | off |
| `prune_disk_pressure` | maintenance | Prune on disk pressure | prune images/volumes when disk crosses a threshold | off |
| `verify_backup` | maintenance | Verify backups | verify backup integrity + notify | off |
| `capacity_warn` | maintenance | Capacity warning | forecast-based capacity alerts | off |

All default **disabled**; destructive ones (`auto_update_containers`,
`prune_unused_volumes`, `patch_critical_cves`, `prune_disk_pressure`) stay off
until an admin opts in. Each run is audited and failures fire a notification.

## Catalog entry (code) + DB override
Code defines: `key, label, description, category, defaultIntervalSeconds, configSchema/defaults`. DB stores only user overrides.

Migration `00041_automations.sql`:
```sql
CREATE TABLE automations (
  key              TEXT PRIMARY KEY,
  enabled          INTEGER NOT NULL DEFAULT 0,
  interval_seconds INTEGER NOT NULL DEFAULT 3600,
  config_json      TEXT NOT NULL DEFAULT '{}',
  last_run         TIMESTAMP,
  last_status      TEXT NOT NULL DEFAULT '',   -- "" | ok | error | skipped | running
  last_detail      TEXT NOT NULL DEFAULT ''
);
```
Store methods: `ListAutomations`, `GetAutomation`, `UpsertAutomation(key,enabled,interval,config)`, `SetAutomationRun(key,status,detail,ranAt)`.

## Engine (`backend/automation`)
- `Engine` with a registry `map[key]Handler`, each handler `func(ctx) (detail string, err error)` closing over the services it needs (injected in `main.go`).
- `Run(ctx)`: ticks every 60s; for each enabled automation whose `last_run + interval` is due, set status `running`, execute, persist `last_status`/`last_detail`/`last_run`, audit (`automation.run`), notify on error. Sequential per tick (no overlap). Respects context cancel.
- `RunNow(ctx, key)`: manual trigger (same execution path).
- Safety: destructive automations enforce the same gating as their manual counterpart (e.g. `auto_remediate_low` refuses anything `ClassifyRisk` != low; `auto_update_containers` only touches projects in its config allowlist).

## API (handlers in `backend/api/automations.go`, routes in `router.go`)
```
GET  /api/automations                 -> { automations: AutomationView[] }   (read; admin gate in handler)
PUT  /api/automations/{key}           body { enabled?, interval_seconds?, config? }  (audited, requireAdmin)
POST /api/automations/{key}/run       manual trigger (audited, requireOperator)
```
`AutomationView`:
```json
{ "key":"restart_unhealthy", "label":"...", "description":"...", "category":"self_heal",
  "enabled":false, "interval_seconds":3600, "config":{}, 
  "last_run":"2026-05-31T...Z"|null, "last_status":"ok", "last_detail":"restarted 2 containers" }
```
New activity actions: `automation.config` (= "Automation configured"), `automation.run` (= "Automation ran"); target type `automation` (add `TargetAutomation="automation"`).

New feature flag (optional gate for the whole surface): `feature.automations` (default true) in `backend/features/features.go` — page shows "not configured" if off.

## Frontend
- `frontend/src/pages/Automations.tsx`: cards grouped by category; each card = toggle (enabled), interval selector, minimal config fields, last-run status badge + timestamp + detail, and a "Run now" button (operator). Admin-only edits (toggle/interval/config); operator can Run now.
- `frontend/src/lib/api/automations.ts`: `useAutomations()`, `useUpdateAutomation()`, `useRunAutomation()` + types (define locally, don't bloat types/api.ts).
- `App.tsx`: route `/automations` (AuthGuard).
- `Sidebar.tsx`: add "Automations" leaf in the **Operations** nav group (operator-visible; gate by `feature.automations` if present).

## Files
- Backend: new `backend/automation/*`, `backend/api/automations.go`, `backend/db/migrations/00041_automations.sql`, store interface + sqlite impl, `backend/activity/actions.go`, `backend/server/router.go`, `backend/main.go` (wire engine + `go engine.Run(ctx)`), `backend/features/features.go` (flag), tests.
- Frontend: `Automations.tsx`, `lib/api/automations.ts`, `App.tsx`, `Sidebar.tsx`.
