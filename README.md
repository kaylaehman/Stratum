<p align="center">
  <img src="assets/banner.png" alt="Stratum — Unified infrastructure management" width="640">
</p>

<p align="center">
  <a href="LICENSE"><img alt="License: AGPL v3" src="https://img.shields.io/badge/license-AGPLv3-blue.svg"></a>
  <img alt="Status" src="https://img.shields.io/badge/status-pre--MVP-orange.svg">
  <img alt="Go" src="https://img.shields.io/badge/go-1.23-00ADD8.svg">
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
- **Agent is optional.** SSH-only works. The agent unlocks richer ops (inotify file watch, ACL reads, cron + systemd timer edits, SSH key audit) but the platform degrades gracefully without it.

## Status

**Pre-MVP.** Active development on the foundation (backend, agent, frontend skeleton, proto layer) and Phase 1 features.

Working today:
- Node registration + auto-detection (Proxmox / standalone Docker / SSH-only)
- Resource tree skeleton
- Filewatch panel per node
- Per-container SSO passthrough config
- 2FA (TOTP RFC 6238) with recovery codes and login gate

MVP build order: node registration → resource tree → filesystem browser → UID/GID conflict visualizer → "why is this broken?" diagnostic → unified log viewer → bind mount tracer → exposed ports + privileged container audit → activity log. Everything else is Phase 2+.

## Quickstart

### Run the dev stack with Docker

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up
```

Backend on `:8080`, frontend (Vite, hot reload) on `:5173`.

### Run locally (two terminals)

Requires Go 1.23+, Node 24+, and [Task](https://taskfile.dev).

```bash
task backend:run        # :8080
task frontend:dev       # :5173
```

### Tests

```bash
task test               # backend + agent, race detector
task frontend:check     # tsc + lint
```

### Add a node

Once the UI is up, register a Linux host. Stratum will probe it in order: Proxmox API → Docker daemon → SSH-only, then surface the detected capabilities so you can confirm or override.

For richer features (file watch, cron edits, SSH key audit), drop the agent on the host with the one-liner the UI generates per node.

## Architecture

| Layer | Tech |
|---|---|
| Frontend | React 19 + TypeScript, Tailwind, Vite |
| Backend API | Go (REST + WebSocket) |
| Agent | Single Go binary, gRPC over mTLS |
| Database | SQLite (single-user) or PostgreSQL (RBAC) |
| Auth | JWT + TOTP 2FA; optional OAuth / SSO passthrough |
| Deploy | Docker Compose |

```
Browser (React)
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
agent/      Per-host Go binary (fs, cron, inotify watch, SSH key audit)
frontend/   React + TS UI (tree, filesystem, diagnostics, logs, security)
proto/      gRPC proto definitions shared between agent and backend
docs/       Specs, ADRs, design notes
assets/     Brand assets
```

## Non-goals

Kubernetes orchestration, cloud-provider integration (AWS/GCP/Azure), full Ansible Tower replacement, long-term log storage (ELK/Loki), image building, DNS management, Windows host support.

## License

[GNU AGPLv3](LICENSE). If you run a modified version as a network service, your modifications must be available to its users under the same license.
