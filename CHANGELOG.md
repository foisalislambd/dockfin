# Changelog

All notable changes to Dockfin will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

Versioning: `X.Y.Z` in [VERSION.md](VERSION.md).
A push to `main` creates a GitHub Release and Docker tags only when that version is higher than the latest `vX.Y.Z` tag.
The same version, or a lower one, does not create a release. Git tag, GitHub Release, and Docker image tags share that `X.Y.Z`.

## [Unreleased]

### Added

- Traefik maintenance page for applications and services. Containers keep running. Caddy keeps the setting and reports that the page is applied only on Traefik
- Permanent (301) www redirects for applications and service domains. Temporary (302) stays the default
- Container file browser for applications, services, and databases: list, read, write, mkdir, and delete text files up to 512 KiB
- Redis, KeyDB, and Dragonfly `FLUSHALL`, confirmed in the API
- API token pause, resume, and roll. A paused token cannot authenticate
- Server health checks for reachability (alert after two failures, and again when it recovers), disk use, an outdated Traefik image, and pending OS patches. The same events were already selectable in Notifications
- Optional commit SHA and message on deployment emails
- Traefik catch-all for unknown hosts (503, or a redirect). Existing servers stay off so an upgrade does not change the edge
- Detect the instance public IPv4/IPv6 from Settings
- `dfin` commands: `tokens`, `maintenance`, `files`, `flush-cache`, and `server-health`
- Queryable audit log for owners and admins: auth, deploys, rollbacks, freezes, settings, member and role changes, and resource create/delete. Filter and paginate `GET /api/v1/audit-logs`. Retention is `DOCKFIN_AUDIT_RETENTION_DAYS` (default 90, `0` keeps rows). Unknown sign-in attempts are stored without a team and shown to every owner and admin
- `PATCH /api/v1/team/members/{userID}` changes a member role. The last owner cannot be demoted
- Optional `DOCKFIN_METRICS_TOKEN`. When set, `GET /metrics` requires `Authorization: Bearer`. Unset keeps the endpoint public
- `DOCKFIN_RATE_LIMIT_STORE=postgres` shares the rate-limit budget across API replicas. The default remains the in-process window, which is also the fallback if Postgres cannot answer
- `dfin` resource commands for servers, projects, environments, applications, services, databases, env vars, and service domains, plus `--json` and `logs --follow`
- Live deploy progress on the deployment page (replayed log history, stage chips, failure text, one-click rollback) and a Content-Security-Policy that still allows the dashboard, Google fonts, and the web terminal
- `scripts/e2e-smoke.sh` boots Postgres, migrates, and exercises health, CSP, metrics auth, register, and the audit log
- Team pulse at `GET /api/v1/overview`: fleet counts, the last 7 days of deploys, queue depth, recent deployments, frozen environments, and attention items (failed deploys, unhealthy resources, failed backups)
- Environment deploy freeze. New application, service, preview, webhook, MCP, and rollback deploys are refused until an owner or admin unfreezes the environment. Owners and admins can override one manual deploy. Webhooks are skipped instead of retried. Restarts, stops, and scheduled commands keep running
- Dashboard, environment, application, and service screens for the pulse and the freeze
- OpenAPI 3.1 generated from the live router, served at `/openapi.json` and `/api/v1/openapi.json`, with an HTML index at `/api/docs`. `packages/openapi/openapi.yaml` is checked in and a test fails when it drifts
- Global per-IP rate limit plus tighter auth, webhook, and sentinel buckets. `429` responses include `Retry-After`
- Prometheus metrics at `GET /metrics` (`dockfin_http_requests_total`, duration sum, in-flight) and structured request logs
- `dfin` commands for freeze, unfreeze, overview, rollback, and service deploy. Commands exit non-zero on HTTP errors
- Home checklist when the team has no projects yet
- Notification event `deploy_freeze` when an environment is paused

### Changed

- New notification channels include `deploy_freeze` in their default event list. Existing channels keep their saved selection until you opt in
- Deployment list endpoints no longer return log payloads. Open a deployment to read its logs
- Project and environment lists compute emptiness in a single query
- `/health` includes the running version
- `dfin` exits 1 when the API returns an HTTP error

### Security

- API responses send `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `X-Frame-Options: SAMEORIGIN`, and a Content-Security-Policy
- Panics return the same JSON error shape as other API failures
- Per-IP rate limits on the whole API, with tighter windows for auth and webhooks. The existing failed-login throttle still applies

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
