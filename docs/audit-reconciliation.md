# Audit Reconciliation — Stratum (Beta → 1.0)

**Wave 0 deliverable.** Maps every audit finding to current code and the feature-flag
catalog (`backend/features/features.go`), classifying each as **CONFIRMED BUG**,
**ALREADY FIXED**, **STALE DATA**, or **CONSOLIDATION**. Produced by a 4-agent ruflo
swarm (hierarchical; flag-catalog / automations-count / bug-triage / baseline+consolidation),
synthesized and spot-verified by hand. **No fixes applied yet — this is the stop-and-review
checkpoint.**

Branch: `wave-0-reconciliation`. Live homelab untouched (analysis only).

---

## 0. Baseline — CLEAN ✅

| Check | Result |
|---|---|
| `task frontend:check` (tsc + eslint) | **Pass, clean** — no errors |
| `task test` (backend + agent) | **Pass** — all 55 backend pkgs + agent pkgs green |
| Race detector | Not run locally (needs cgo/gcc; unavailable on this Windows host). **CI runs `-race` and is authoritative** — last CI on `main` (commit `633ba3c`) was green. |

Baseline is clean before any Wave work begins.

---

## 1. Automations count — TRUE NUMBER IS **15** (single-source it)

Ground truth: `backend/automation/catalog.go` defines **15** entries (`var catalog []Entry`);
`handlers.go` registers 15 keys; `Engine.ListViews()` returns all 15 unfiltered. The three
displayed numbers diverge because two are hand-maintained literals, not derived from the catalog.

| Surface | Shows | What it actually counts | Classification |
|---|---|---|---|
| `/automations` page | **15** | `catalog` via `ListViews` (no filter, not seed rows) | **CORRECT** ✅ |
| Features panel ("8") | **8** | Hardcoded literal in the flag *description* string, `features.go:26` | **CONFIRMED BUG** (stale literal) |
| README line 110 ("13") | **13** | Hand-written prose, no backing artifact | **STALE DATA** (prose) |
| `docs/contracts/automations-contract.md` | 8 | Original design spec, never updated | **STALE DATA** (doc) |
| `backend/server/automations_test.go:48` | asserts 8 | Frozen at original count; currently *skips* (engine not wired in test server) so it hides the drift | **CONFIRMED BUG** (latent test) |

**Decision: 15 is canonical; derive every surface from `len(automation.Catalog())`** (exported at
`catalog.go:163`).
- `features.go:26` — inject catalog size at `main.go` wire-up, or drop the count from the description (the `/automations` page already shows it). *(Touches the flag catalog — see §2.)*
- `README.md:110` — 13 → 15.
- `automations_test.go:48` — assert `len(automation.Catalog())` instead of `8`.
- contract doc — list all 15, header 8 → 15 (docs-only).

---

## 2. Feature-flag catalog reconciliation

13 flags in `backend/features/features.go` (lines 26–38). The `Default` bool is the only
code-level maturity signal; the README's own rule (README.md:83) is *default=true → Stable*.

| Flag | Default | Code tier | README tier | Agree? | Finding |
|---|---|---|---|---|---|
| `feature.reverse_proxy` | true | Stable | Stable | ✅ | — |
| `feature.dns_management` | true | Stable | Stable | ✅ | — |
| `feature.cert_management` | true | Stable | Stable | ✅ | (but see Bug C1) |
| `feature.health_check_editor` | true | Stable | Stable | ✅ | — |
| `feature.wake_on_lan` | true | Stable | Stable | ✅ | — |
| `feature.action_2fa` | true | Stable | Stable | ✅ | (but see Bug C5) |
| `feature.ai_agent` | true | Stable | Stable | ✅ | — |
| `feature.sso_passthrough` | false | Planned | Planned | ✅ | "not yet implemented" |
| `feature.chat_integration` | false | Planned | Planned | ✅ | "not yet implemented" |
| `feature.automations` | **true** | **Stable** | **Beta** | ❌ | **Tier drift** — code says Stable, README lists under Beta. Also holds the stale "8" literal. |
| `feature.config_versions` | true | Stable | *(none)* | ❌ | On by default, **undocumented** in README + FEATURES.md flag table |
| `feature.alert_policies` | true | Stable | *(none)* | ❌ | On by default, **undocumented** |
| `feature.config_git` | false | Planned | *(none)* | ❌ | Off by default, "not yet implemented", **undocumented** |

**Also:** five README *Beta* subsystems — agentic remediation, security posture score, incident
timeline, uptime monitoring, stacks edit — have **no backing feature flag at all** (not toggleable).
Relevant to Wave 3 (graduating Beta → Stable): decide whether each should get a flag or is
permanently-on infrastructure.

**Classification:** all CONSOLIDATION/STALE-DATA (doc drift), not code bugs — except the
`feature.automations` description literal (CONFIRMED BUG, §1).

---

## 3. Confirmed-bug triage (Wave 1)

| # | Finding | Classification | Root cause / location | Fix direction |
|---|---|---|---|---|
| C1 | Cert monitor reports `ACCVRAIZ1` on all nodes | **CONFIRMED BUG** (corrected from agent's "already fixed") | `scanPaths` includes `/etc/ssl` (`certs/service.go:30`). `leafCert` returns the *first* cert of a file — correct for a fullchain, but for the system bundle `/etc/ssl/certs/ca-certificates.crt` the first cert is a CA root. Same bundle on every node → identical ACCVRAIZ1. | Exclude system trust-store bundles (drop `/etc/ssl` → `/etc/ssl/private` or filter files; and/or skip self-signed CA roots `IsCA && Subject==Issuer` when recording serving certs). A `Rescan All` then flushes stale rows. **Isolated commit (security).** |
| C2 | Posture/Security page hangs ~10s on first load, instant on Rescan | **CONFIRMED BUG** | `Privileged`/`Ports` handlers call `ensureSecurityForDockerNodes` **synchronously** in the GET hot path (`api/security.go:67–115`); cold cache → serialized per-container Docker inspects (N+1). Rescan feels instant only because the cache is warm by then. | Serve cached immediately; refresh async (`go ensure…` on a background timer / startup seed). Don't block GET on a full scan. |
| C3 | Stacks editor: "No compose file could be located" for running/imported stacks | **CONFIRMED BUG** | `FindCompose` aborts when `projectContainerNames` returns empty (`stacks/service.go:139`) — true for imported stacks or ones whose containers were removed. The label path is otherwise correct. | Decouple discovery from live container presence: use the DB-persisted compose path / `working_dir` label even when no containers are currently listed. Fail with the *actual* reason if truly absent. |
| C4a | Stack row expands only via chevron | **CONFIRMED BUG** (UX) | Header `div` has no `onClick`; only the chevron `<button>` toggles (`Stacks.tsx:1692`). | Move toggle to the header row + `cursor:pointer`; `stopPropagation` on action buttons. |
| C4b | Container names look clickable, aren't | **CONFIRMED BUG** (UX) | Plain `<span>` styled like a link (`Stacks.tsx:1768`); only the ExternalLink icon navigates (and only to `/resources`). | Wrap name in a link to the container detail, or visually de-emphasize. |
| C4c | Bulk Ops action bar scrolls off-screen | **CONFIRMED BUG** (UX) | Action bar in normal flow below an `overflow:auto` table, no sticky (`Bulk.tsx:590`). | `position:sticky; bottom:0; zIndex:10` (or flex-pin). |
| C4d | Skills "N issues" reads as alarms | **CONFIRMED BUG** (UX, minor) | `issue_count` is a count of catalogued `common_issues` YAML entries, not live findings (`Skills.tsx:172`). | Relabel to "N guides"/"N patterns"; hide at 0. |
| C5 | Admin step-up 2FA **fails open** with no TOTP enrolled | **CONFIRMED BUG** (security — highest priority) | `requireStepUp` returns `true` when `!TwoFA.Enabled(user)` (`api/authz.go:57`); `Enabled` is false when the user has no TOTP row (`twofa/service.go:184`). So an admin who never enrolled TOTP bypasses step-up on **every** destructive action. SECURITY.md claims "admin role + TOTP step-up" required → documented control not enforced. | Fail **closed** for admin on destructive/step-up actions when no TOTP enrolled (return `totp_enrollment_required`), and prompt enrollment. Do not silently change other auth. **Isolated commit; preserve gate + audit.** |

---

## 4. Consolidation map (Wave 2) — overlaps over the same data

| Area | Current | Realistic reduction |
|---|---|---|
| Container surfaces | Stacks, Bulk Ops, Update Assistant — all read `useTree()` (`/api/tree`). Bulk + Stacks are the same inventory; Updates has its own `/api/updates` digest shape. | Fold **Bulk Ops into Stacks** as a select/bulk-action toolbar → −1 route, −1 duplicate `ContainerTable`. Keep Updates separate (different data). |
| CVE scheduling | `SchedulesPanel` in CVE page (`/api/security/cve/schedules`, own table) **and** `scheduled_cve_scan` automation — genuine duplication. | Pick Automations as canonical; CVE page links to it. Needs `config.targets` on the automation for per-node/container granularity. −~200 LOC, −4 endpoints, −1 table. |
| Backup scheduling | Backups page = manual trigger only; recurring lives in Automations (`scheduled_backups`). | **Correctly separated — no change.** |
| Node context | **No global node in Topbar** (confirmed `Topbar.tsx`). 4+ local `<select>` reimplementations (Terminal, Backups×2, CVE). | Brief wants one global top-bar node context; agent argues per-page is contextually correct (Terminal/CVE act on one target). **→ design decision for you (§6).** At minimum: one shared `NodeSelector`/`ContainerSelector`. |
| Knowledge surfaces | Skills, Runbooks (embedded in `/assistant`), per-Skill Common Issues — overlapping schemas; a Common Issue **cannot** become a Runbook today. | Low-risk win: **"Promote to Runbook"** button on a Common Issue (no schema change). Optional: surface Runbooks as their own route. |

---

## 5. Net classification summary

- **CONFIRMED BUGS (fix in Wave 1):** C1 cert (security), C2 posture hang, C3 stacks compose, C4a–d UX, C5 admin 2FA fail-open (security), automations "8" literal + latent test.
- **ALREADY FIXED:** cert *leaf-selection* logic itself (correct for fullchains); the older `{{.Label}}` compose-discovery bug.
- **STALE DATA:** README "13", contract-doc "8", undocumented flags (config_versions / alert_policies / config_git) — doc/seed drift, not code defects.
- **CONSOLIDATION:** Bulk↔Stacks, CVE-schedule↔Automations, node selectors, Skills↔Runbooks.

---

## 6. Open decisions for you (before Wave 1 fixes)

1. **C5 admin 2FA** — confirm the desired behavior: fail **closed** for admins on destructive
   actions + force TOTP enrollment (recommended; matches SECURITY.md), vs. relax SECURITY.md to
   say step-up is opt-in. This changes auth, so I won't pick for you.
2. **C1 cert scope** — OK to narrow cert scanning away from system trust bundles (`/etc/ssl`)?
   This is the right fix but slightly changes what the cert page lists (roots disappear, which is
   the point).
3. **Automations "8" literal** — inject `len(Catalog())` into the flag description, or just drop
   the parenthetical count? (Editing `features.go` touches the flag catalog.)
4. **Node context (Wave 2)** — global top-bar node (your brief) vs. shared per-page selector
   component (agent's recommendation, since Terminal/CVE genuinely act on one target)?
5. **Beta-without-flags** — should agentic remediation / posture / incident timeline / uptime /
   stacks-edit each get a real feature flag (so Wave 3 can graduate them by flipping defaults), or
   stay permanently-on?

**Recommended Wave 1 order:** C5 (security, isolated) → C1 (security, isolated) → C2 (perf) →
C3 (stacks) → C4a–d (UX, batchable) → automations count single-source. Each with tests; security
fixes in their own commits per guardrail #2/#3.
