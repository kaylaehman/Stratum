# Security Policy

## Threat Model

Stratum is a self-hosted platform. The operator controls the host and is
responsible for network-perimeter security. The threat model is scoped to what
Stratum itself guarantees — not what the underlying host OS or network provides.

### What Stratum trusts

| Trust boundary | Assumption |
|---|---|
| Operator / Admin role | Fully trusted. Admin users have full platform access. |
| JWT sessions | Signed with `JWT_SECRET` (min 32 bytes). Compromise of the secret allows session forgery. |
| Node SSH credentials | Stored **encrypted at rest** in `nodes.credentials_encrypted` — AES-256-GCM sealed with the same `ENCRYPTION_KEY`/cipher as the secrets vault (`NodeCredentials.Seal`). Private keys, passwords, and tokens are decrypted into memory only at connection time, and are never returned over the API or written to logs/activity. They are NOT stored in plaintext, but they ARE persisted (encrypted) — so the vault key is what protects them. |
| Agent mTLS certificates | Issued by an internal CA at agent registration time. The CA private key is operator-controlled. |
| Secrets vault (AES-256) | Encrypted at rest using `ENCRYPTION_KEY` (32-byte AES-256 GCM). Raw secrets are never logged or written to disk outside the vault. |

### What Stratum does not trust

- **Network-adjacent clients** — all API routes require a valid JWT session except
  `/health`, the first-run setup endpoints (`/api/setup/status`, `/api/setup/admin`),
  and the auth endpoints (`/api/auth/login`, `/api/auth/refresh`, which uses the
  refresh cookie). No unauthenticated data-plane access. This is enforced
  structurally by `TestAllRoutesRequireAuth`, which walks the whole route table.
- **Operator- or AI-generated commands** — every remediation command is classified
  by a risk engine before it reaches the SSH layer. The engine uses a positive
  allowlist (not a denylist). Any command not explicitly recognized as safe
  defaults to `High` risk and requires TOTP step-up before execution. Commands
  with shell metacharacters (`;`, `|`, `&&`, `` ` ``, `$(`) default to
  `Destructive` and require admin role plus step-up. The generate-then-approve
  workflow ensures no command auto-executes.
- **Container images** — images are scanned for known CVEs (via Trivy/Grype)
  before deployment and on digest change. Results are stored per-digest and
  surfaced in the UI.
- **Inbound file content** — file uploads and editor saves are gated by path
  validation and the activity log middleware; no writes bypass audit.
- **Indirect code execution** — scripts (`./script.sh`, `bash script.sh`,
  `sh -c '...'`), interpreter one-liners (`python3 -c`, `python -c`), and
  orchestration tools (`ansible-playbook`) are all classified High by the
  remediation engine because they are opaque to static analysis. They cannot
  auto-execute and require TOTP step-up.

### Separation of duties

- **Viewer** — read-only; no mutations.
- **Operator** — can start, stop, restart containers and view logs; cannot
  approve destructive proposals or manage users.
- **Admin** — full access, including destructive remediation approval, RBAC
  management, and secret reveal.

Role enforcement is implemented in the backend — frontend checks are
defense-in-depth only.

### Fail-closed authorization

All API routes are protected by auth middleware that rejects requests with
missing or invalid JWTs. Missing role in a resource-scoped check defaults to
deny, not allow. If `RequiresStepUp` is called with an unrecognized risk level
it returns `true` (step-up required), not `false`.

### Audit log

Every mutating action (file write, permission change, container lifecycle,
secret access, remediation execution, RBAC change) is appended to an
append-only activity log in the database. The log cannot be modified or deleted
through the API; rows are inserted only, never updated or soft-deleted.

### Secret material handling

- `ENCRYPTION_KEY` (AES-256 secrets-vault key) should be supplied via
  `ENCRYPTION_KEY_FILE` pointing to a Docker/Kubernetes secret mount, so it
  does not appear in `docker inspect` output or the process environment.
  The raw `ENCRYPTION_KEY` env var is supported for back-compat and dev
  environments.
- `JWT_SECRET` must be at least 32 bytes. Values shorter than this are rejected
  at startup.
- Neither `ENCRYPTION_KEY` nor `JWT_SECRET` is ever logged or included in
  error messages.
- Secrets stored in the vault are decrypted only on explicit user action (the
  "reveal" flow), which is itself gated by re-authentication and logged.
- Neither `ENCRYPTION_KEY` nor `JWT_SECRET` appearing in startup validation
  errors is enforced by `TestConfigErrorsNeverEchoSecrets`.

### Blast radius — if the Stratum host is compromised

This is the question to ask first, and the honest answer matters more than any
control table. **Stratum is a high-value target by design: it holds the keys to
every node it manages.** If an attacker gains code execution on the Stratum host
*and* can read the `ENCRYPTION_KEY`, they can decrypt:

- every node's SSH credentials (private keys, passwords) in
  `nodes.credentials_encrypted`,
- every stored API token (Proxmox, Cloudflare, registry, AI providers),
- the entire AES-256 secrets vault.

That is effectively lateral movement to every managed node. The encryption at
rest protects against database theft *without* the key (a stolen `.db` file or
backup is useless alone); it does **not** protect against a full host compromise
where the key is also readable.

Recommended mitigations (defense in depth — Stratum cannot enforce these for you):

- **Isolate the host.** Put Stratum on a management VLAN; do not expose `:8080`
  to the internet. Front it with a reverse proxy + your own auth if remote access
  is required.
- **Supply the key out-of-band.** Use `ENCRYPTION_KEY_FILE` pointing at a secret
  mount (Docker/K8s secret, tmpfs) rather than the raw `ENCRYPTION_KEY` env var,
  so the key is not in `docker inspect`, the process environment, or shell
  history. Keep it off the same disk as the database backups.
- **Least-privilege SSH per node.** Give Stratum a dedicated, non-root SSH
  account per node with only the access each feature needs, rather than a shared
  root key. A host compromise then yields scoped accounts, not root everywhere.
- **Restrict and rotate.** Rotate node credentials and `ENCRYPTION_KEY` if the
  host is ever suspected compromised; revoke the agent CA and re-enrol.
- **Back up the database encrypted and off-host**, and never store the key
  alongside the backup.

---

## Supported Versions

Only the latest release on the `main` branch is actively supported for security
fixes. Older tagged releases receive no backports.

---

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

### Private Vulnerability Reporting (preferred)

Use GitHub's built-in Private Vulnerability Reporting:

1. Go to the repository on GitHub.
2. Click **Security** > **Report a vulnerability**.
3. Fill in the advisory form. Include reproduction steps, affected version,
   and an assessment of impact if you have one.

GitHub will notify the maintainer privately. Expect an acknowledgment within
**5 business days** and a status update within **14 days**.

### Alternative contact

If you are unable to use GitHub's advisory flow, email:
**security@kaylas.systems** — include "STRATUM SECURITY" in the subject line.
PGP is not currently required; plain-text email is fine.

### Disclosure timeline

| Step | Target |
|---|---|
| Initial acknowledgment | 5 business days |
| Triage and severity assessment | 14 days from report |
| Fix development | Severity-dependent (Critical: 7 days, High: 30 days) |
| Coordinated public disclosure | After fix is released; reporter credited unless anonymity requested |

---

## Security Posture Summary

| Control | Implementation |
|---|---|
| Authentication | JWT (RS256 or HS256), configurable expiry |
| Secrets at rest | AES-256-GCM, key from `ENCRYPTION_KEY` / `ENCRYPTION_KEY_FILE` |
| Agent channel | gRPC over mTLS (CA-issued certs) |
| Proxmox TLS | Certificate verification **on** by default; `InsecureSkipVerify` is per-node opt-in for homelab self-signed certs and must be enabled explicitly per endpoint |
| AI egress | Dial-time SSRF guard blocks private/link-local/metadata addresses; loopback (local Ollama) allowed, LAN hosts via `STRATUM_EGRESS_ALLOW_HOSTS` |
| Metrics endpoint | `/metrics` requires `STRATUM_METRICS_TOKEN` bearer, or loopback-only when unset |
| Brute-force defense | Per-IP rate limit on login and AI-ask |
| Remediation gating | Positive allowlist + TOTP step-up (fail-closed) |
| Audit trail | Append-only activity log, all mutations |
| CVE scanning | Trivy (preferred) or Grype, per image digest |
| RBAC | Admin / Operator / Viewer, backend-enforced |
| Privilege escalation defense | Destructive remediation requires Admin role + step-up |
