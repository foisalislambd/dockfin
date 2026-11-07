# Changelog

All notable changes to Dockfin will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

Versioning: `X.Y.Z` in [VERSION.md](VERSION.md).
A push to `main` creates a GitHub Release and Docker tags only when that version is higher than the latest `vX.Y.Z` tag.
The same version, or a lower one, does not create a release. Git tag, GitHub Release, and Docker image tags share that `X.Y.Z`.

## [Unreleased]

## [2.1.0] - 2026-11-07

### Added

- Durable background jobs for on-demand image scans, Docker cleanup, and instance backups. `GET /api/v1/jobs`, `GET /api/v1/jobs/{id}`, and `POST /api/v1/jobs/{id}/cancel`. The worker claims with `FOR UPDATE SKIP LOCKED`, retries with backoff, and drops a job after three attempts. `dfin jobs` shows the same queue
- `GET /ready` for process readiness (database ping)
- Project grants now cover environment variables (including reveal), resource create, scheduled tasks, deployment cancel, resource moves, deploy-by-id for a signed-in member, and the MCP tools

### Changed

- On-demand scans, Docker cleanup, and “Run backup now” return `202` and finish in the job worker. The dashboard waits for the job and shows the error
- Production (`DOCKFIN_ENV` other than development) refuses `GET /metrics` unless `DOCKFIN_METRICS_TOKEN` is set. Development still serves metrics without a token
- `DOCKFIN_PUBLIC_URL` must be an absolute `http` or `https` URL or the process refuses to start
- A deployment is claimed only while it is still `queued`, so two workers cannot both start it
- Git tree scans and registry tag lookups from the control plane use the same private-network block as notification webhooks
- The website workflow uses one Pages concurrency group so two pushes to `main` cannot sit on the same GitHub Pages deployment slot
- Dashboard actions that previously swallowed failures (server validate, proxy start, API token pause/roll/revoke, GitLab hook delete, vulnerability dismiss, certificate regenerate, server export, proxy config delete) now show the error

### Security

- See [SECURITY_AUDIT.md](SECURITY_AUDIT.md) for the 2.1.0 review: project-scope bypasses, control-plane SSRF, metrics exposure, and the checks that were already sound

### Included from main after v2.0.0

These shipped on `main` after the v2.0.0 tag and are part of 2.1.0.

- Newer image tags for a scanned image: patch and line candidates, registry digests when the registry returns them, and a confirmed UPGRADE or REBUILD that deploys the application
- Same-instance server transfer moves applications, services, and databases with the server. A shared project is split. Type MOVE to confirm
- Image vulnerability scans with Syft and Grype on the server that holds the image, stored findings, an on-demand scan, and a deploy gate (off, warn, or block)
- Passphrase-encrypted server bundles that move the private key, environment variables, shared variables, and database credentials to another Dockfin
- Swarm stacks for Caddy and Traefik, with the maintenance page running as a Swarm service
- Project access for members: every project, or view / deploy / manage on chosen projects. Owners and admins are not scoped. Hidden projects return 404
- Passkeys (WebAuthn) for registration and sign-in. Two-factor authentication still runs when it is enabled. The relying party is `DOCKFIN_PUBLIC_URL`
- Hostinger: validate a token, list KVM plans and virtual machines, link an existing VPS, or purchase one after typing `PURCHASE`. Purchase charges the Hostinger account
- GitLab system hooks for push and merge request events, matched to applications by `path_with_namespace`
- Repository smart scan that suggests dockerfile, railpack, static, or compose from the git tree
- CloudWatch log drains through the existing Vector container
- Docker attach on the terminal (`docker logs` then `docker attach`, detach with Ctrl-p Ctrl-q)
- Fleet page with the latest CPU, memory, and disk sample per server
- Metadata export and import that does not include the private key
- Caddy maintenance page and unknown-host catch-all, using the same nginx page as Traefik
- `dfin fleet`, `dfin scan`, and `dfin transfer`
- Traefik maintenance page for applications and services. Containers keep running. Caddy serves the same page from its Caddyfile
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
