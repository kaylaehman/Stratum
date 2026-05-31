# Part C — contracts + integration map

Conventions: response envelope `writeJSON`/`writeError`; handler-level auth gates
(`requireAdmin`/`requireOperator`/`requireStepUp`); audited mutations under the
`audited` router group; React Query hooks in `frontend/src/lib/api/*.ts`.

## Migration number assignments (last existing = 00041)
- C1 backup restore/verify metadata → `00042_backup_restore.sql`
- C3 config versioning → `00043_config_versions.sql`
- C5 secret expiry/rotation → `00044_secret_expiry.sql`
- C6 alert policies → `00045_alert_policies.sql`
- C9 push subscriptions → `00046_push_subscriptions.sql`
- C2 orchestration (drain history, optional) → `00047_orchestration_runs.sql`
- C4 forecast, C7 drexport, C10 runbook-validate → NO migration (read existing).

## Integration files owned by the ORCHESTRATOR (agents must NOT edit; return snippets instead)
`backend/server/router.go`, `backend/activity/actions.go`, `backend/api/handlers.go`,
`backend/main.go`, `backend/features/features.go`, `backend/automation/catalog.go`
(+ handlers.go for verify_backup/capacity_warn), frontend `App.tsx` + `Sidebar.tsx`.
Agents define a NARROW store interface IN THEIR OWN package (only the methods they
need) and implement them on `*sqlite.Store` in a NEW file `db/sqlite/<feature>.go`
— do NOT add to the central `db.Store` interface (it breaks every fake).

---

## C2 — Dependency-aware orchestration (ordering contract)
Package `backend/orchestration`. Reads `depgraph` edges + `nodeconn`/`docker`/
`proxmox` lifecycle.

**Ordering rule:** build the dependency DAG from depgraph edges (container→network,
container→volume, and explicit `depends_on` where available).
- **Start**: dependencies UP first → topological order (a node's deps before it).
- **Stop**: reverse topological (dependents down before their deps).
- **Restart**: stop-order then start-order.
- **Cycles**: detect SCCs; within a cycle, act in arbitrary stable order and log a
  `cycle` warning (don't deadlock). Per-step timeout (default 60s); on a step
  failure, stop the sequence and report which step failed (no auto-rollback).

**Types:**
```go
type Step struct { Kind string; ID string; Name string; Action string; Order int }
type Plan struct { Target string; Action string; Steps []Step; Cycles [][]string }
type StepResult struct { Step Step; OK bool; Error string; DurationMs int64 }
```
**API** (audited; `requireOperator` for start/restart, `requireAdmin`+`requireStepUp` for stop-all/drain):
```
POST /api/orchestration/plan      body {target_kind:"stack"|"node", target_id, action, dry_run:true}  -> Plan
POST /api/orchestration/execute   body {target_kind, target_id, action}                                -> {results:StepResult[]}
POST /api/nodes/{id}/drain        body {dry_run?}  -> Plan (dry_run) or {results} — stop guests/containers in dep order before reboot
```
`dry_run` returns the Plan without acting. Audit action `orchestration.execute` / `node.drain`.

---

## C6 — Alert policy / notification routing (policy schema)
Package `backend/alertpolicy`. Sits between `incident`/webhook emit and channel
dispatch (`webhooks`/notifications). An emit calls `alertpolicy.Route(ctx, Alert)`
which decides: which channels, suppress (dedup/quiet-hours), or escalate.

**Alert input:** `{Key string (dedup key, e.g. trigger+target); Severity string (info|warning|critical); Title; Text; Source}`.

**Policy (DB-backed, admin-editable):**
```go
type Policy struct {
  ID string; Name string; Enabled bool;
  MinSeverity string;                 // info|warning|critical — drop below this
  Channels []string;                  // webhook ids this policy routes to
  Match struct{ Sources []string; KeyGlob string }  // optional filters
  QuietHours *struct{ StartMin, EndMin int; Tz string; AllowCritical bool }  // minutes-of-day
  DedupWindowSec int;                 // suppress same Key within window (0=off)
  Escalate *struct{ AfterSec int; Channels []string }  // re-route if unacked after AfterSec
}
```
**Routing semantics:** for each enabled policy whose Match + MinSeverity admit the
alert: if within DedupWindowSec of the last delivery for this Key → suppress (count
it). If in QuietHours and not (critical && AllowCritical) → suppress. Else deliver
to Channels. Record deliveries/suppressions in a `alert_deliveries` table for the UI
and dedup state.

**Migration `00045_alert_policies.sql`:** `alert_policies` (policy rows, JSON for
nested config) + `alert_deliveries` (id, policy_id, alert_key, severity, channel,
status: delivered|suppressed_dedup|suppressed_quiet, created_at).

**API** (admin; audited): `GET /api/alert-policies`, `POST /api/alert-policies`,
`PUT /api/alert-policies/{id}`, `DELETE /api/alert-policies/{id}`,
`GET /api/alert-deliveries?limit=` (read). Default policy seeded: route everything
to all channels (back-compat — current fire-hose behavior until the user customizes).

**Feature flag:** `feature.alert_policies` default true.

---

## Automation catalog additions (orchestrator applies after C1/C4 land)
- `verify_backup` (C1, category maintenance) — restore newest archive to scratch, checksum/stat, pass/fail.
- `capacity_warn` (C4, category maintenance) — fire when a projected time-to-threshold is within the horizon.
