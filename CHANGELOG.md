# Changelog

All notable changes to Dockfin will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

Versioning: `X.Y.Z` in [VERSION.md](VERSION.md).
A push to `main` creates a GitHub Release and Docker tags only when that version is higher than the latest `vX.Y.Z` tag.
The same version, or a lower one, does not create a release. Git tag, GitHub Release, and Docker image tags share that `X.Y.Z`.

## [Unreleased]

## [1.0.0] - 2026-07-31

### Added

- Go control plane (`dockfin serve` / `migrate`) with Chi HTTP API
- React + Vite dashboard (auth, servers, projects, apps, databases, services)
- PostgreSQL schema via Goose migrations (embedded)
- SSH pool with host-key TOFU / fingerprint persistence
- Deploy pipeline: dockerfile, dockercompose, dockerimage, railpack, static
- Deployment queue with cancel, concurrency limits, SSE logs, rollback
- Env vars + shared env resolution at deploy time
- Git webhooks with HMAC verification and PR preview hooks
- Managed database start/stop over SSH; S3 / scheduled backup APIs
- One-click service catalog loader (Coolify-compatible compose templates)
- Notifications (Discord / Slack / webhook) on deploy finish/fail
- `dfin` CLI (`health`, `apps`, `servers`, `deploy`, `logs`)
- `scripts/install.sh` and `scripts/vps-oneclick-test.sh`
- Release pipeline: tagged `vX.Y.Z` → GHCR images + GitHub Release (same version)
- Open-source project docs (CONTRIBUTING, SECURITY, CODE_OF_CONDUCT, SUPPORT)

### Security

- Argon2id password hashing; AES-256-GCM secret box for keys and env values
- Session cookies + bearer tokens; webhook signature verification when configured
