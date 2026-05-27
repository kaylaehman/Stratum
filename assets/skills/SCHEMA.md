# Stratum Skill Library — File Schema

This document defines the YAML file format for Stratum's bundled skill library
(`assets/skills/{category}/{container-name}.yaml`). Skills are runbook-style
diagnostic and remediation procedures the AI agent uses to triage common
container issues.

The on-disk YAML format is intentionally richer than the runtime `Skill`
struct in `FEATURES.md` F9. The loader projects each `common_issues[]` entry
into a runtime `Skill` and attaches container-match metadata for triggering.

## File Naming

```
assets/skills/{category}/{container-name}.yaml
```

- Lowercase kebab-case file names — `nginx-proxy-manager.yaml`,
  `home-assistant.yaml`.
- No spaces, no underscores in file names.
- One file per container; multi-container stacks (e.g. mailcow) get one file
  representing the primary service.

## Category List

```
media | files | ai | automation | network | monitoring | security |
productivity | development | database | backup | finance | photos |
ebooks | data | games | communication | documents | web | dashboard |
rss | weather | time | passwords | voip | smarthome | analytics |
email | identity | torrent
```

Add a new category only after discussion — the agent UI groups skills by
this field.

## Top-Level Schema

```yaml
id: unique-kebab-case-id        # globally unique across the library
name: Human readable name
version: "1.0"                  # bump on breaking changes to steps/commands
container_match:
  image_patterns:               # substring match against container image
    - "jellyfin/jellyfin"
    - "linuxserver/jellyfin"
  port_hints: [8096]            # optional, used as a tiebreaker
category: media                 # one of the categories above
description: One-line purpose statement
docs_url: https://...           # official docs or canonical Docker Hub page

common_issues:                  # see "common_issues" section below
  - id: ...

default_config:                 # minimal compose snippet + variable contract
  compose_template: |
    services:
      jellyfin:
        image: jellyfin/jellyfin:latest
        restart: unless-stopped
        ...
  required_vars:
    - name: PUID
      description: User ID to run as
      default: "1000"
  known_mounts:
    - container_path: /config
      purpose: Configuration and database files
      recommended_permissions: "755"
      owner: "{PUID}:{PGID}"

linuxserver_conventions:
  enabled: false                # true for linuxserver.io images
  puid_env: PUID
  pgid_env: PGID
  config_mount: /config

notes: |
  Free-form gotchas, first-run quirks, version notes.
```

## `common_issues[]` Schema

Each entry is a complete runbook for one observable failure mode.

```yaml
common_issues:
  - id: permission-denied-config     # unique within this file
    name: Permission denied on /config mount
    symptoms:                         # human-readable signs the user might see
      - "permission denied"
      - "EACCES"
      - "Cannot create directory"
    trigger_conditions:               # machine-checkable conditions
      - log_pattern: "permission denied"
      - exit_code: 1
    steps:
      - id: step-1
        description: Check current ownership of the host config path
        type: check                   # check | fix | inform | confirm
        command: "stat {config_path}"
        expected_output: "Uid: {puid}"
        on_fail: step-2               # next step if expected_output mismatched
      - id: step-2
        description: Recursively chown the config path to PUID:PGID
        type: fix
        command: "chown -R {puid}:{pgid} {config_path}"
        requires_approval: true       # ALWAYS true on type: fix
        on_success: step-3
    resolution: |
      Plain-English explanation of what was wrong and what the fix did.
      Shown to the user after successful resolution.
```

### Step `type` values

| type | Purpose | `requires_approval` |
|---|---|---|
| `check` | Read-only diagnostic. Run automatically. | not required |
| `fix` | Mutating action. Must be approved by user. | **always `true`** |
| `inform` | Display a message to the user (no command). | not required |
| `confirm` | Explicit user confirmation gate before next step. | not required |

### Branching

- `on_success: <step-id>` — next step on success (default: next in array)
- `on_fail: <step-id>` — next step on failure (default: abort the runbook)

### Variable Substitution

Steps may reference `{variable_name}` placeholders. The loader fills these
from (in order of precedence):

1. Values supplied by the AI agent at runtime (e.g. resolved `{puid}` from
   container inspect)
2. `default_config.required_vars[].default`
3. Sensible built-ins: `{config_path}`, `{mount_path}`, `{container_name}`

Unknown variables abort the runbook with a clear error.

## `common_issues` Minimums

| Container class | Minimum entries |
|---|---|
| Standard container | 2 |
| Well-known (Jellyfin, Plex, Nextcloud, Immich, Home Assistant, Authentik, Vaultwarden, Portainer, n8n, Gitea, PostgreSQL, MariaDB, Paperless-ngx, Frigate, Traefik) | 4 |
| Permission-complex (any LinuxServer.io image, Nextcloud, Immich, PhotoPrism) | At least 1 entry must cover UID/GID issues |

## Safety Rules

1. **Every `type: fix` step MUST set `requires_approval: true`.** No exceptions.
2. Never include destructive commands (`rm -rf`, `DROP TABLE`, `docker volume rm`)
   without an explicit `type: confirm` step immediately preceding them.
3. Network-changing commands (firewall, DNS, port bindings) require a
   `type: inform` step before the `type: fix` step explaining the impact.
4. No command may write outside the container's known mount paths without an
   `inform + confirm` pair first.

## Compose Template Rules

- Minimal — strip to essential services, volumes, env vars. Not a copy of the
  upstream's full `docker-compose.yml`.
- All user-supplied values use `${VARIABLE}` substitution.
- Always include `restart: unless-stopped`.
- Volumes prefer host bind mounts (`./config:/config`) so file ownership is
  visible to the host — Stratum's UID/GID visualizer relies on this.

## LinuxServer.io Conventions

For any `linuxserver/*` image:

- Set `linuxserver_conventions.enabled: true`.
- Document `PUID`, `PGID`, `TZ` as required vars.
- `known_mounts` must include `/config` with `owner: "{PUID}:{PGID}"`.
- Include at least one `common_issues` entry covering UID/GID drift.

## Mapping to F9 `Skill` Struct

At load time, each `common_issues[]` entry becomes one runtime `Skill`:

```
Skill {
  id:                  "{file.id}/{issue.id}"
  name:                "{file.name} — {issue.name}"
  description:         issue.name + " (auto-generated from " + file.id + ")"
  trigger_conditions:  issue.trigger_conditions
  steps:               issue.steps (with variable substitution applied)
  requires_approval:   true if any step has type: fix, else false
  created_by:          "stratum-bundled"
}
```

The container-level fields (`container_match`, `default_config`,
`linuxserver_conventions`) are stored as **skill-set metadata** keyed by file
id, so the AI can:

- Auto-suggest only skills whose `container_match.image_patterns` match the
  current container's image
- Use `default_config` to scaffold a fresh deployment of this container
- Use `known_mounts` to drive the UID/GID visualizer's "expected owner"
  highlight

## Versioning

- `version: "1.0"` on every file at initial creation.
- Bump `version` when any step's `command` changes meaning, when fields are
  renamed, or when removing a `common_issues` entry. Adding new entries is
  not a breaking change.

## Adding a New Skill

1. Pick the right category (or propose a new one in PR description).
2. Copy a sibling file in the same category as a template.
3. Verify the image name and default ports against the official docs — do not
   guess. Add the `docs_url`.
4. Write at least 2 `common_issues` (4 for well-known containers).
5. Lint: run `task skills:lint` (validates schema + safety rules).
6. Add an entry to `INDEX.md` (or regenerate via `task skills:index`).
