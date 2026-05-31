<p align="center">
  <img src="assets/banner.png" alt="Stratum — Unified infrastructure management" width="640">
</p>

<p align="center">
  <a href="LICENSE"><img alt="License: AGPL v3" src="https://img.shields.io/badge/license-AGPLv3-blue.svg"></a>
  <img alt="Status" src="https://img.shields.io/badge/status-beta-yellow.svg">
  <a href="https://github.com/kaylaehman/Stratum/pkgs/container/stratum-backend"><img alt="GHCR" src="https://img.shields.io/badge/images-ghcr.io-2496ED.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/go-1.26-00ADD8.svg">
  <img alt="React" src="https://img.shields.io/badge/react-19-61DAFB.svg">
</p>

# Stratum

Self-hosted, unified infrastructure management for the messy middle — bridging hypervisors, VMs, bare Linux servers, and Docker containers into one pane of glass. Built for homelabs and small teams where no single tool (Portainer, Cockpit, the Proxmox UI) covers the full stack.

Navigate host → VM → container → filesystem → file permissions in a single UI, with deep diagnostic tooling for the permission conflicts, bind-mount weirdness, and silent access failures that eat hours of debugging time.

## What makes it different

- **No Proxmox required.** Proxmox is one supported node type — not a prerequisite. Stratum runs fully against any Linux host reachable via SSH or agent: Ubuntu, Debian, RHEL/Rocky/Alma, Arch, Alpine.
- **Auto-detection.** Add a host; Stratum probes Proxmox API → Docker socket → SSH and classifies the node. You can override.
- **"Why is this broken?" diagnostic.** Pick a file and a container; get a plain-English explanation of why the container process can or cannot access it (UID/GID resolution, bind-mount tracing, ACLs, root vs non-root, suggested `chmod`/`chown`).
- **UID/GID conflict visualizer.** Side-by-side host vs container user tables with mismatch highlighting — the #1 cause of silent Docker bind-mount failures.
- **Agent is optional.** SSH-only works. The agent unlocks richer ops (real-time inotify file watch over mTLS gRPC, init-system detection) and the platform degrades gracefully without it.

## Quickstart

One command against the published images — no build step.

```bash
# 1. Grab the compose file and the env template
curl -O https://raw.githubusercontent.com/kaylaehman/Stratum/main/docker-compose.yml
curl -O https://raw.githubusercontent.com/kaylaehman/Stratum/main/.env.example
cp .env.example .env

# 2. Set the two required secrets in .env
#    openssl rand -base64 48   ->  JWT_SECRET
#    openssl rand -hex 32      ->  ENCRYPTION_KEY

# 3. Up
docker compose up -d
```

The UI is on <http://localhost:8080>. The images are multi-arch (amd64 + arm64) and published to GHCR:

- `ghcr.io/kaylaehman/stratum-backend`
- `ghcr.io/kaylaehman/stratum-frontend`

Pin a release in production (recommended): `STRATUM_TAG=v0.1.0 docker compose up -d`.

Then register a Linux host from the UI — Stratum probes Proxmox API → Docker daemon → SSH-only and surfaces the detected capabilities to confirm or override. For richer features (real-time file watch), drop the agent on the host with the one-liner the UI generates per node.

> **Build from source instead?** `docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build`

## Status

**Beta.** The MVP and most of Phase 2 are shipped and running in production deployments. The platform is usable end-to-end; newer subsystems (listed under *Beta* below) are functional but less battle-tested, and a few capabilities are still *Planned*.

Feature maturity is grounded in the feature-flag catalog (`backend/features/features.go`): flags shipped on-by-default are Stable, flags that are off-by-default are Planned. The toggle list is visible (and editable by admins) under **Settings → Features**.

### Stable

Core navigation and the diagnostics that are the reason this exists:

- Node registration + auto-detection (Proxmox / standalone Docker / SSH-only)
- Unified resource tree (host → VM/LXC → container → filesystem)
- Filesystem browser + config editor (SFTP/agent), syntax highlighting, diff-on-save
- UID/GID conflict visualizer · "Why is this broken?" diagnostic · bind-mount tracer
- Unified multi-container log viewer · resource timeline (CPU/RAM/IO history)
- Exposed-ports audit · privileged-container flags · image CVE scan (Trivy) · SSH-key audit · file-change detection
- Container lifecycle · bulk operations · update assistant · snapshots + rollback · dependency graph · network topology
- Volume health · template library · secrets manager (AES-256) · scheduled tasks (cron + systemd timers) · backup orchestration
- Smart search · bookmarks · activity log · notification hooks (Slack / Discord / Telegram) · script & Ansible runner · multi-user RBAC
- Reverse-proxy detection (incl. **cloudflared**: host-service, containerized, and dashboard-managed tunnels) · DNS record view · certificate monitoring · health-check editor · Wake-on-LAN · step-up 2FA (TOTP) · AI assistant

### Beta

Shipped and working, newer / hardening:

- **Agent (gRPC over mTLS)** — real-time inotify file-watch streaming + init-system detection
- **Agentic remediation** — propose → approve → execute host fixes, risk-classified with a positive low-risk allowlist; anything not allowlisted requires TOTP step-up; destructive needs admin
- **Security posture score** — A–F per node composed from CVE / privileged / exposed-port / stale-key / outdated-image signals
- **Incident timeline** — "what changed?" across the activity log, metric spikes, restarts, and file events
- **Uptime monitoring** — HTTP/TCP/ICMP checks with history and uptime %
- **Stacks edit & redeploy** — edit live Compose stacks with Secrets-backed env injection (never written to disk)

### Planned

- **Automations engine** — user-configurable autonomous self-healing & update tasks (auto-restart, auto-update, auto-remediate, etc.) built on the remediation engine, each behind its own risk ceiling + kill-switch
- **SSO passthrough** — auth in front of containers (`feature.sso_passthrough`, off by default)
- **Inbound chat command integration** (`feature.chat_integration`, off by default)
- Longer term: Kubernetes, cloud-provider integrations

## Architecture

| Layer | Tech |
|---|---|
| Frontend | React 19 + TypeScript, Tailwind, Vite (installable PWA) |
| Backend API | Go (REST + WebSocket) |
| Agent | Single Go binary, gRPC over mTLS |
| Database | SQLite (single-user) or PostgreSQL (RBAC) |
| Auth | JWT + TOTP 2FA; optional OAuth / SSO passthrough |
| Deploy | Docker Compose against published GHCR images |

```
Browser (React PWA)
    ↕  WebSocket + REST
Backend (Go)
    ├── Proxmox REST API     (proxmox nodes only)
    ├── Docker Engine API    (standalone + proxmox)
    ├── SSH / SFTP           (all node types)
    └── Agent gRPC / mTLS    (when installed)
```

## Repository layout

```
backend/    Go API server, Proxmox/Docker/SSH clients, scheduler, secrets
agent/      Per-host Go binary (mTLS gRPC server, inotify watch, init detection)
frontend/   React + TS PWA (tree, filesystem, diagnostics, logs, security)
proto/      gRPC proto definitions shared between agent and backend
docs/       Specs, ADRs, design notes
```

## Releases

Images are built and published automatically when a semver tag is pushed (`.github/workflows/release.yml`): multi-arch build → GHCR → GitHub Release. To cut one:

```bash
git tag v0.1.0 && git push origin v0.1.0
```

## Non-goals

Kubernetes orchestration, cloud-provider integration (AWS/GCP/Azure), full Ansible Tower replacement, long-term log storage (ELK/Loki), image building, Windows host support.

## License

[GNU AGPLv3](LICENSE). If you run a modified version as a network service, your modifications must be available to its users under the same license.
