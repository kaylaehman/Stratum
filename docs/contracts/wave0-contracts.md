# Wave 0 — Shared contracts (v3 swarm brief)

Authoritative request/response + route shapes for the v3 build-out. Implementer
agents MUST conform to these so front-end and back-end halves agree without
serializing. Conventions in this repo:

- Response envelope: plain JSON. Success = the value or `{"key": [...]}`; error =
  `{"error": "<code>"}` via `writeError` (`backend/api/response.go`).
- Auth gates live **in the handler** (`requireAdmin` / `requireOperator` /
  `requireStepUp` in `backend/api/authz.go`), not the route. Audited mutations are
  registered under the `audited := r.With(mw.Activity(...))` group in
  `backend/server/router.go` and set `activity.FromContext(r.Context())` fields.
- Frontend: React Query hooks in `frontend/src/lib/api/*.ts`, TS types in
  `frontend/src/types/api.ts`. No raw fetch in components.

---

## C0. Resources deep-link route (consumed by A2, A8, A9) — ALREADY EXISTS

`frontend/src/pages/Resources.tsx` → `useResourceDeepLink()` (≈line 856) already
reads query params, selects + expands the target in the tree store, then strips
the params (`setParams({}, {replace:true})`).

**Canonical shape:**
```
/resources?node=<nodeId>                      → selects the host node
/resources?node=<nodeId>&container=<containerId>  → selects + reveals the container
```
- `<containerId>` is the inventory container id (`Container.id` from `/api/tree`,
  i.e. `node.containers[].id`) — NOT the docker_id.
- It expands `node:<nodeId>` and `project:<nodeId>:<projectKey>` (projectKey =
  `compose_project` or the sentinel `" ungrouped"`).

**Implication:** A2 "step 1" (Resources accepts a target) is DONE. The "blank
Resources" bug is purely that callers navigate to `/resources` with **no params**.
Fixes are caller-side: pass `?node=&container=`. A8 and A9 deep-link the same way.
Do NOT invent a new route param scheme — reuse this.

Helper to add (frontend, A2 owns it): a tiny builder so callers don't hand-format:
```ts
// frontend/src/lib/resourceLink.ts
export function resourceLink(nodeId: string, containerId?: string): string {
  const p = new URLSearchParams({ node: nodeId })
  if (containerId) p.set('container', containerId)
  return `/resources?${p.toString()}`
}
```

---

## C1. A4 — Volume bulk-prune (unused)

Server **recomputes** the unused set (never trusts a client-supplied name list).
Reuses `volumes.Service` classification (`status()` → `"unused"`) and the existing
`docker.Client.RemoveVolume`.

**Route (audited group):**
```
POST /api/volumes/prune-unused
```
- Gate: `requireAdmin` + `requireStepUp` (matches single-volume `RemoveVolume`).
- Request body: `{ "node_id"?: "<id>" }` — omit/empty = every docker-capable node.
- Behavior: enumerate unused volumes via the volumes service (per node), call
  `RemoveVolume(name, force=false)` for each, collect per-volume results. A volume
  that flips to in-use between list and delete returns `ok:false` with its error;
  never fail the whole batch.
- Response `200`:
```json
{
  "results": [
    { "node_id": "n1", "name": "old_data", "ok": true },
    { "node_id": "n1", "name": "stuck", "ok": false, "error": "volume_in_use" }
  ],
  "removed_count": 1,
  "failed_count": 1
}
```
- Audit: one event, `action = volume.prune_unused` (new constant), target_type
  `volume`, detail `{node_id?, removed_count, failed_count}`.

**Frontend (`Volumes.tsx`, A4-frontend):**
- Add an `unused`-only filter (segment/toggle) over the existing `VolumeView[]`.
- "Remove all unused (N)" button (admin only) → confirm dialog listing the N
  unused volumes → `POST /api/volumes/prune-unused` → on settle, show per-volume
  success/failure and invalidate the volumes query.
- Hook: `usepruneUnusedVolumes()` in `frontend/src/lib/api/volumes.ts`.
- Types: `PruneUnusedResponse { results: {node_id,name,ok,error?}[]; removed_count; failed_count }`.

---

## C2. A5 — Updates reason aggregation

Keep `GET /api/updates` response shape (`{"updates":[updateView...]}`) and ADD a
`summary` object. Add a backend reason **category** classifier so the UI can tell
"locally built" from "can't check".

**Backend (`backend/updates`):** add
```go
// Category buckets a raw UnknownReason for UI aggregation.
func Category(unknownReason string) string // one of the constants below
const (
  CatLocallyBuilt        = "locally_built"        // "no repo digest (locally-built…)"
  CatRegistryUnreachable = "registry_unreachable" // "registry lookup failed: …" (network/egress)
  CatAuth                = "auth"                  // registry auth required (401/403 in error)
  CatRateLimited         = "rate_limited"         // Docker Hub 429 / toomanyrequests in error
  CatDaemonError         = "daemon_error"         // "local digest unavailable: …"
  CatEmptyDigest         = "empty_digest"         // "registry returned empty digest"
  CatOther               = "other"
)
```
`Category` inspects the persisted `UnknownReason` string (and substring-matches
`401/403/unauthorized` → auth, `429/toomanyrequests/rate` → rate_limited).

**`GET /api/updates` response (extended):**
```json
{
  "updates": [ /* unchanged updateView[] */ ],
  "summary": {
    "total": 12,
    "up_to_date": 3,
    "update_available": 1,
    "unknown": 8,
    "dominant_unknown_category": "locally_built",
    "dominant_unknown_count": 6,
    "unknown_breakdown": [
      { "category": "locally_built", "count": 6, "example_reason": "no repo digest (locally-built or never pushed)" },
      { "category": "rate_limited",  "count": 2, "example_reason": "registry lookup failed: toomanyrequests" }
    ]
  }
}
```
`summary` computed in the handler from the rows. `dominant_*` = the highest-count
category among unknowns (empty/0 when no unknowns).

**Frontend (`Updates.tsx`, A5-frontend):** when `summary.unknown > 0` and
dominates, show an aggregated banner keyed on `dominant_unknown_category` with a
remediation hint (locally_built → "these are built locally, nothing to check";
rate_limited → "Docker Hub anonymous rate limit — add registry auth";
registry_unreachable → "backend can't reach the registry — check egress"). Keep
the per-row tooltip. Types: extend `UpdatesResponse` with `summary?: UpdatesSummary`.

---

## C3. A8 — Cloudflared route target → resource resolution

Enrich each proxy `Rule` in the `proxy.Status` response with an optional resolved
target. `Rule` lives in `backend/proxy/adapter.go` (shared by all adapters); add an
**optional** field so other adapters are unaffected.

**Backend (`backend/proxy`):**
```go
// ResolvedTarget links a rule's TargetURL to a known container/resource.
type ResolvedTarget struct {
  NodeID      string `json:"node_id"`
  ContainerID string `json:"container_id"` // inventory id → resourceLink()
  Name        string `json:"name"`
  MatchKind   string `json:"match_kind"`   // "container_name" | "localhost_port" | "host_ip_port"
}
// added to Rule:
//   Resolved *ResolvedTarget `json:"resolved,omitempty"`
```
Resolution runs in `Service.Status` AFTER `ListRules`, only when rules exist.
Match `TargetURL` (parse host:port) against containers on this node + reachable
nodes:
- host ∈ {localhost,127.0.0.1,::1} + port → container on THIS node publishing
  `HostPort == port` → `localhost_port`.
- host is a name (non-IP) → container whose `Name == host` (this node first) →
  `container_name`.
- host is an IP + port → container on the node whose host IP == that IP,
  publishing that port → `host_ip_port`.
First match wins; unresolved → `Resolved` nil. Published-port source: prefer the
existing security ports index if available, else live `docker inspect` ports
(`docker.PortBinding`). Add table tests for the three match kinds.

**Frontend (`ReverseProxyPanel.tsx`, A8-frontend):**
- Resolved target → clickable link `resourceLink(resolved.node_id, resolved.container_id)`.
- `SourceHost` → external link to `https://<source_host>`.
- Unresolved target → plain text + subtle "not found" hint.
- Read-only state copy (A8 part 3): when `detected==='cloudflared'` and host file
  access is unavailable, render ONE explanatory line ("cloudflared ingress is read
  from the on-disk config / Cloudflare dashboard; reading it needs SSH credentials
  for this node") with a link to add SSH credentials for the node (Nodes page /
  node edit). Not a generic error. Keep capabilities read-only.

---

## C4. A9 — Stack lifecycle + create

### C4a. Stack lifecycle (stop/start/restart a whole project)
**Route (audited):**
```
POST /api/nodes/{id}/stacks/{project}/lifecycle    body: { "action": "stop|start|restart" }
```
- Gate: `requireOperator` (matches container start/stop/restart). Audited.
- Behavior: run `docker compose -p <project> <action>` over the node connection
  (`fs.Service.Exec`). If the compose path is discoverable, prefer
  `docker compose -f <path> <action>`; else fall back to acting on every container
  carrying the `com.docker.compose.project=<project>` label. Reuse
  `stacks.FindCompose`. `project` sanitized via `sanitizeProject`.
- Response `200`: `{ "action": "stop", "project": "<p>", "output": "<combined>" }`.
- Audit: `action = stack.<action>` (new constants `ActionStackStop/Start/Restart`)
  or a single `stack.lifecycle` with detail `{action}`; target_type `stack`.

**Frontend:** stack/project node in the resource tree
(`frontend/src/components/tree/ResourceTree.tsx`) gets Stop/Start/Restart actions
(operator only) with a confirm; on success invalidate `TREE_KEY`. Also surface on
the `Stacks.tsx` `StackCard`.

### C4b. Create a stack (choose node)
**Route (audited):**
```
POST /api/nodes/{id}/stacks    body:
  { "project": "<name>", "directory": "/opt/<name>",
    "compose_yaml": "<yaml>", "env_vars": [EnvVar...], "secret_groups": ["..."] }
```
- Gate: `requireAdmin` (writes new files + deploys — matches `RedeployStack`). Audited.
- Validation: `project` non-empty, `sanitizeProject(project)` must equal it (reject
  names that change under sanitization); `directory` absolute, no `..`, default
  `/opt/<project>`. Server creates the dir (`fs.Service.Mkdir`/exec `mkdir -p`),
  writes `<directory>/docker-compose.yml`, then `stacks.Deploy(...)` (which already
  injects secrets at deploy, never to disk). Reject if a compose file already
  exists there (no silent overwrite) unless `project` already known.
- Response `200`: `{ "project": "<name>", "compose_path": "<dir>/docker-compose.yml", "output": "<deploy output>" }`.
- Audit: `action = stack.create` (new constant), target_type `stack`,
  detail `{node_id, project, directory}`.

**Frontend (`Stacks.tsx`, A9-frontend):** "New stack" flow — pick target node
(filter `capabilities.docker`), project name, directory (default `/opt/<name>`),
compose YAML (optionally seeded from a Template via the existing render endpoint),
optional secret-group injection. On success, select the new project. Hook
`useCreateStack()` + `useStackLifecycle()` in `frontend/src/lib/api/stacks.ts`.

---

## New activity action constants (backend/activity/actions.go)
- `ActionVolumePruneUnused = "volume.prune_unused"`
- `ActionStackCreate = "stack.create"`
- `ActionStackStop = "stack.stop"`, `ActionStackStart = "stack.start"`, `ActionStackRestart = "stack.restart"`

## Files each Wave-1/2 item owns (disjoint check)
- A4-backend: `backend/api/volumes.go`, `backend/docker/volumes.go` (helper), `backend/volumes/*` (reuse), `backend/server/router.go` (one line), actions.go
- A5-backend: `backend/updates/*`, `backend/api/updates.go`
- A8-backend: `backend/proxy/*`, `backend/api/proxy.go`
- A9-backend: `backend/stacks/*`, `backend/api/stacks.go`, `backend/server/router.go` (two lines), actions.go
- Shared hotspots `router.go` + `actions.go`: A4 + A9 both append lines. To avoid
  a collision, ONE agent (A9-backend) owns all router.go + actions.go edits for
  Wave 1; A4-backend hands its route/const to A9 or appends in a later serialized
  step. (Enforced by the orchestrator, not the agents.)
```
