# Contributing to Stratum

Thank you for contributing. This file covers the conventions and workflows that
keep the codebase consistent. See `CLAUDE.md` for the full project overview and
`SKILLS.md` for the skill library schema.

---

## General guidelines

- Go: explicit error handling over panics; no unhandled promise rejections in JS.
- All filesystem write operations must go through the activity-log middleware.
- Secret values are never stored in plaintext; encrypt before DB write, decrypt
  only on explicit reveal.
- Functions under 20 lines; typed interfaces for all public APIs.
- Dark mode via Tailwind `dark:` classes; no inline theme-color styles.

## Running tests

```sh
# Backend (from backend/ directory)
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./...

# Skills catalog validation specifically
GOWORK=off go test ./skills/... -run TestValidateCatalog -v -count=1
```

CI runs `go vet`, `staticcheck`, and `go test` on every push to `main`. The
push triggers a Docker image rebuild and redeploy of the live instance.

---

## Adding a service skill

A **skill** teaches the AI assistant how to diagnose and remediate common
problems for one container image. See `SKILLS.md` for the full schema and
`assets/skills/SCHEMA.md` for the authoritative on-disk format.

### Step-by-step

1. **Pick a category.** Check the existing directories under `assets/skills/`.
   Use an existing category or, if none fits, propose a new one in your PR
   description (a new directory + a new entry in `AllowedCategories` in
   `backend/skills/validate.go`).

2. **Choose a filename.** Use lowercase kebab-case matching the container's
   canonical name: `home-assistant.yaml`, `nginx-proxy-manager.yaml`. No
   spaces, no underscores. The `id` field in the file **must match the filename
   stem exactly** (enforced by the validator).

3. **Copy a sibling file** in the same category as a starting template. Remove
   the issues that do not apply and fill in real data.

4. **Verify image names and ports** against the official documentation or
   Docker Hub page — do not guess. Set `docs_url` to the official docs.

5. **Write at least 2 `common_issues`** (4 for well-known containers; see
   `SKILLS.md` for the full minimum table). Each issue must have at least one
   `check` or `fix` step.

6. **Follow the safety rules for steps:**

   | Step type | requires_approval | Notes |
   |---|---|---|
   | `check` | not required | Read-only diagnostic. |
   | `fix` | **always `true`** | Any omission is a hard validation error. |
   | `inform` | not required | Message display only; no command. |
   | `confirm` | not required | Gate before a destructive fix. |

   Never include a destructive command (`rm -rf`, `docker volume rm`,
   `DROP TABLE`) without a `confirm` step immediately preceding it.

7. **Run the validator locally:**

   ```sh
   cd backend
   GOWORK=off go test ./skills/... -run TestValidateCatalog -v -count=1
   ```

   The `task skills:validate` task (when wired) should run the same command.
   Fix every reported violation before opening a PR.

8. **Update `assets/skills/INDEX.md`** by hand: add a row to the category
   table for your new skill. (`task skills:index` is planned but not yet
   implemented.)

### Minimal valid skill template

```yaml
id: my-service               # matches filename stem: my-service.yaml
name: My Service
version: "1.0"
category: web                # one of the AllowedCategories enum values
description: One-line purpose statement.
docs_url: https://docs.example.com/

container_match:
  image_patterns:
    - "myorg/my-service"
  port_hints: [8080]

common_issues:
  - id: cannot-start
    name: Container exits immediately on start
    symptoms:
      - "exited with code 1"
      - "panic"
    trigger_conditions:
      - exit_code: 1
    steps:
      - id: step-1
        description: Inspect recent container logs
        type: check
        command: "docker logs {container_name} --tail 30"
        expected_output: ""
      - id: step-2
        description: Display log output for review
        type: inform
        command: ""

  - id: permission-denied-config
    name: Permission denied on /config mount
    symptoms:
      - "permission denied"
      - "EACCES"
    trigger_conditions:
      - log_pattern: "permission denied"
    steps:
      - id: step-1
        description: Check ownership of the config path
        type: check
        command: "stat {config_path}"
        expected_output: "Uid:"
        on_fail: step-2
      - id: step-2
        description: Show current permissions
        type: inform
        command: ""
      - id: step-3
        description: Chown config path to match container user
        type: fix
        command: "chown -R {puid}:{pgid} {config_path}"
        requires_approval: true

default_config:
  compose_template: |
    services:
      my-service:
        image: myorg/my-service:latest
        restart: unless-stopped
        volumes:
          - ${CONFIG_PATH:-./config}:/config
  required_vars:
    - name: CONFIG_PATH
      description: Host path for the config volume
      default: "./config"
  known_mounts:
    - container_path: /config
      purpose: Application configuration
      recommended_permissions: "755"
      owner: "1000:1000"

linuxserver_conventions:
  enabled: false

notes: |
  Any first-run quirks, version notes, or gotchas for this container.
```

### Good first issues (service skills to add)

The following well-known homelab services are not yet in the catalog and would
make excellent first contributions:

- **Immich** (`photos` category) — permission-complex; needs UID/GID coverage
- **Frigate** (`smarthome` category) — well-known; needs 4+ issues
- **Nextcloud** (`files` or `productivity` category) — permission-complex;
  needs UID/GID and database-connection issues
