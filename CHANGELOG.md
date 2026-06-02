# Changelog

All notable changes to Stratum are documented here. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); dates are ISO 8601.

Security fixes are called out explicitly — a control that has never been tested
against reality is a promise, not a guarantee, and publishing what we fixed is
how you tell the difference.

## [Unreleased]

### Security

- **Admin step-up 2FA no longer fails open.** Destructive actions are gated by a
  TOTP step-up (SECURITY.md), but `requireStepUp` previously returned *allow*
  when the user had no TOTP enrolled — so an admin who never set up 2FA could run
  every destructive action with no challenge. It now fails **closed** (gated by
  the `feature.action_2fa` flag): no enrolment → `428 totp_enrollment_required`
  and the UI prompts enrolment; enrolled-but-stale → `428 2fa_required`. Covered
  by `stepup_enforcement_test.go`.
- **Certificate monitoring reported a trust-store root, not the leaf cert.** The
  scanner read `/etc/ssl` including the system CA bundle
  (`/etc/ssl/certs/ca-certificates.crt`) and reported its first certificate — a
  CA root such as `ACCVRAIZ1` — identically on every node, making expiry
  monitoring meaningless. `leafCert` now skips CA certificates and the scan
  prunes the system trust store. (Run a Rescan to flush stale rows.)
- **Regression tests for SECURITY.md invariants.** Added `TestAllRoutesRequireAuth`
  (every route rejects a tokenless request except the documented public bootstrap
  routes) and `TestConfigErrorsNeverEchoSecrets` (`ENCRYPTION_KEY`/`JWT_SECRET`
  never appear in startup errors). The cert bug proved invariants drift silently
  when only asserted in prose.
- **SECURITY.md reconciled with reality.** Corrected the claim that SSH private
  keys are "never written to the DB" — they are persisted AES-256-GCM-encrypted
  in `nodes.credentials_encrypted` (decrypted in memory only at connect time);
  added a *Blast radius* section stating plainly what a compromise of the Stratum
  host yields (every node's keys + the vault) and the recommended mitigations.

### Fixed

- **Security & posture pages no longer hang ~10s on first load.** The handlers ran
  a full Docker security scan synchronously on a cold cache. The scan is now
  detached to the background (warming the cache for the next load) with a short
  bounded wait; the posture page also drops a wasted per-node SSH round-trip whose
  result was always discarded.
- **Compose stack discovery matches the real Docker project name.** Stacks whose
  project name contains `.`, uppercase, etc. failed with "No compose file could be
  located" because the name was sanitized before the Docker label match. The raw
  name is now used for label matching (shell-quoted, injection-safe); the path
  fallback still sanitizes against traversal.
- **Automations count is single-sourced.** The page showed 15, the Features panel
  said 8, and the README said 13. The true count derives from
  `automation.Catalog()` (15); a parity test keeps the catalog and handlers in
  lock-step.

### Changed (UX)

- Stack rows toggle from anywhere on the row (not just the chevron); container
  names link to the specific container; the Bulk Ops action bar is pinned
  (sticky) so it stays visible while a long selection scrolls; the Skills badge
  reads "N guides" (catalogued troubleshooting guides) instead of an alarm-styled
  "N issues".

---

> **Releases.** Tagging `vX.Y.Z` triggers the GHCR multi-arch build + GitHub
> release. Cut the first tag (`v0.1.0`) once the changes above are merged to
> `main`, so the release contains them.
