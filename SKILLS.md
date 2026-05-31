# Stratum Skill Library

A **skill** is a diagnostic state machine: a self-contained runbook that
teaches the AI assistant how to recognize, diagnose, and remediate one or more
common failure modes for a specific container image.

Each skill is a YAML file at `assets/skills/{category}/{id}.yaml`. The
loader parses all files on startup into an in-memory library (`backend/skills`
package). Skills are read-only reference data served to the AI assistant as
context and to the REST API at `GET /api/skills`.

---

## Mental model: state machine

```
Container symptom / trigger condition
        |
        v
  [CommonIssue]  ← identified problem
        |
        v
    step-1  (type: check) ─── on_fail ──> step-2
                            └─ on_success ─> step-3
        |
        v
    step-2  (type: inform)     ← show message to user
        |
        v
    step-3  (type: fix)        ← mutating action, requires_approval: true
        |
        v
    step-4  (type: check)      ← verify fix succeeded
```

Steps are executed in array order unless `on_fail` or `on_success` redirect
execution. The state machine terminates when: a fix step succeeds, the last
step is reached, or an unhandled `on_fail` fires.

---

## File layout

```
assets/skills/
  {category}/         ← one directory per category
    {id}.yaml         ← one file per container image; id matches filename stem
```

File naming rules:
- Lowercase kebab-case: `nginx-proxy-manager.yaml`, `home-assistant.yaml`
- No spaces, no underscores
- One file per primary service (multi-service stacks use the primary service name)

---

## Category enum

The `category` field must be exactly one of these values (derived from the
on-disk directory names):

```
ai | analytics | automation | backup | communication | dashboard | data |
database | development | documents | ebooks | email | files | finance |
games | identity | media | monitoring | network | passwords | photos |
productivity | rss | security | smarthome | time | torrent | voip |
weather | web
```

Adding a category requires creating the directory AND adding the new value to
`AllowedCategories` in `backend/skills/validate.go`.

---

## Annotated example skill

```yaml
# Top-level identity ─────────────────────────────────────────────────────────
id: postgresql               # unique across the entire catalog; matches filename stem
name: PostgreSQL             # human-readable name shown in UI
version: "1.0"               # bump on breaking changes to steps or commands
category: database           # must be one of the enum values above
description: >-              # one-line purpose statement
  Advanced open-source relational database with JSONB and full-text search
docs_url: https://www.postgresql.org/docs/

# How the loader matches this skill to a running container ───────────────────
container_match:
  image_patterns:            # case-insensitive substring match against image ref
    - "postgres"
    - "postgresql"
  port_hints: [5432]         # tiebreaker; integers 1..65535

# The diagnostic state machines ──────────────────────────────────────────────
common_issues:
  - id: permission-denied-data          # unique within this file
    name: Permission denied on /var/lib/postgresql/data mount
    symptoms:
      - "permission denied"             # human-readable signs the user might see
      - "cannot access directory"
    trigger_conditions:
      - log_pattern: "permission denied" # machine-checkable; matched against logs
      - log_pattern: "EACCES"

    steps:
      - id: step-1
        description: Check ownership of the host data path
        type: check                      # read-only; runs automatically; no approval needed
        command: "stat {data_path}"
        expected_output: "Uid: 999"      # if output does not contain this string, on_fail fires
        on_fail: step-2                  # redirect to step-2 when expected_output mismatches

      - id: step-2
        description: Display current ownership for review
        type: inform                     # show a message; no command executed
        command: ""

      - id: step-3
        description: Recursively chown data path to UID 999 (postgres user)
        type: fix                        # mutating action — MUST set requires_approval: true
        command: "chown -R 999:999 {data_path}"
        requires_approval: true          # user must explicitly approve before execution

    resolution: |
      The official postgres image runs as UID 999. If the host path was created
      as root or another user the container cannot initialize the cluster. The
      fix changes ownership to 999:999.

# Compose scaffold shown when deploying a fresh instance ─────────────────────
default_config:
  compose_template: |
    services:
      postgresql:
        image: postgres:latest
        restart: unless-stopped
        environment:
          POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
        volumes:
          - ${POSTGRES_DATA_PATH:-./data}:/var/lib/postgresql/data
  required_vars:
    - name: POSTGRES_PASSWORD
      description: Password for the superuser (required, non-empty)
      default: ""
    - name: POSTGRES_DATA_PATH
      description: Host path for the data volume
      default: "./data"
  known_mounts:
    - container_path: /var/lib/postgresql/data
      purpose: PostgreSQL cluster data
      recommended_permissions: "700"
      owner: "999:999"            # used by the UID/GID visualizer

# LinuxServer.io convention block (omit or set enabled: false for non-LS images)
linuxserver_conventions:
  enabled: false

notes: |
  Runs as UID 999 (user postgres). Data directory must be writable by this user.
```

---

## Step `type` values

| type | Purpose | requires_approval |
|---|---|---|
| `check` | Read-only diagnostic. Runs automatically. | not required |
| `fix` | Mutating action. Must be approved by user. | **always `true`** |
| `inform` | Display a message; no command executed. | not required |
| `confirm` | Explicit user confirmation gate before next step. | not required |

### Safety rules (enforced by `backend/skills.Validate`)

1. Every `type: fix` step **must** set `requires_approval: true`. No exceptions.
2. A `fix` step whose command `remediation.ClassifyRisk` rates as `destructive`
   and that lacks `requires_approval: true` is a hard validation error.
3. No destructive command (`rm -rf`, `DROP TABLE`, `docker volume rm`) without
   an explicit `confirm` step immediately preceding it.
4. Network-changing commands require an `inform` step before the `fix` step.

---

## Variable substitution

Steps may reference `{variable_name}` placeholders. At runtime the loader
fills them from (in priority order):

1. Values supplied by the AI agent (resolved from container inspect, etc.)
2. `default_config.required_vars[].default`
3. Built-in context: `{container_name}`, `{config_path}`, `{mount_path}`

Unknown variables abort the runbook with a descriptive error.

---

## Branching

- `on_success: <step-id>` — jump to this step on success (default: next in array)
- `on_fail: <step-id>` — jump to this step on failure (default: abort runbook)

References must resolve to a real step `id` within the **same** issue. The
validator catches dangling references.

---

## Validation

The `backend/skills` package exposes:

```go
// Validate checks an already-parsed catalog slice and returns all violations.
func Validate(catalog []Skill) []error

// ValidateFile checks a single skill and additionally verifies that id matches
// the filename stem.
func ValidateFile(sk Skill, path string) []error
```

The rules enforced:

| Rule | Hard error? |
|---|---|
| id, name, category, description, version present | yes |
| category in AllowedCategories | yes |
| id unique across catalog | yes |
| id matches filename stem (ValidateFile only) | yes |
| step.type in {check, fix, inform, confirm} | yes |
| on_fail / on_success resolve within same issue | yes |
| port_hints in 1..65535 | yes |
| fix step missing requires_approval | yes |
| fix step command classified RiskDestructive + no requires_approval | yes |

Run locally from `backend/`:

```sh
GOWORK=off go test ./skills/... -run TestValidateCatalog -v
```

The `task skills:validate` target should run:

```sh
cd backend && GOWORK=off go test ./skills/... -run TestValidateCatalog -v -count=1
```

---

## LinuxServer.io conventions

For any `linuxserver/*` image set `linuxserver_conventions.enabled: true` and:

- Document `PUID`, `PGID`, `TZ` as `required_vars`.
- Include `/config` in `known_mounts` with `owner: "{PUID}:{PGID}"`.
- Add at least one `common_issues` entry covering UID/GID drift.

---

## Minimum `common_issues` count

| Container class | Minimum |
|---|---|
| Standard container | 2 |
| Well-known (Jellyfin, Plex, Nextcloud, Immich, Home Assistant, Authentik, Vaultwarden, Portainer, n8n, Gitea, PostgreSQL, MariaDB, Paperless-ngx, Frigate, Traefik) | 4 |
| Permission-complex (LinuxServer.io, Nextcloud, Immich, PhotoPrism) | at least 1 covering UID/GID |
