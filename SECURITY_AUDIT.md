# Security audit — 2.1.0

Review of the control plane, dashboard, `dfin`, deploy path, and GitHub Actions as of this release. Findings below are fixed in this version unless the row says otherwise.

## Fixed

| Area | What was wrong | Fix |
| --- | --- | --- |
| Project ACL | `enforceProjectACL` only matched `/api/v1/{projects,applications,services,databases,environments,deployments}/{uuid}`. A member limited to specific projects could still reveal environment variables, create resources, run scheduled tasks (including commands on the server), cancel deployments, move a resource, deploy by id while signed in, and use the MCP tools. | `requireResourceAccess` / `allowResource` check the resource’s project. Create, move, scheduled tasks, env vars, deployment cancel, session deploy-by-id, and MCP reads and deploys use it. A denial is the same 404 as a missing resource. Webhook secrets and API tokens stay team-scoped; the extra check runs for a session cookie. |
| SSRF | Git smart-scan and registry tag lookup used a plain HTTP client, so the control plane could be pointed at link-local or metadata addresses. | Both use `notify.Client` with the same policy as notification webhooks: private, loopback, link-local, and metadata addresses are refused unless allowlisted. Docker pull on the target server is unchanged. A private Git host scanned from the panel needs an allowlist entry. |
| Metrics | An empty `DOCKFIN_METRICS_TOKEN` left `/metrics` public, including in production. | Production refuses `/metrics` until the token is set. Development and tests with no config still serve it. Startup logs a warning when production has no token. |
| Deploy claim | Two workers could both observe a queued deployment and start it. | `ClaimDeployment` updates only a row that is still `queued`. |
| Server transfer | After a move, `scheduled_backups.s3_storage_id` was cleared but the replica rows in `scheduled_backup_s3_storages` could still point at the old team’s storage. Health and metrics team updates ignored errors. | Replica rows for those backups are deleted. The team updates return their errors. |
| Scheduler cancel | `Stop` cancelled the loop, but backup, task, and cleanup work used `context.Background()`. | Those goroutines use the loop context. |
| Job work on the request | On-demand image scans, Docker cleanup, and instance backup ran inside the HTTP handler (SSH and dumps on the request context). | They enqueue a Postgres job and return `202`. See below. |

## Background jobs

Deploys already have a durable `deployments` row, an in-memory dispatcher, and crash reclaim. Replacing that queue was not useful. Transfers are one database transaction and stay synchronous. Audit retention and sentinel sampling stay on the scheduler.

On-demand scans, Docker cleanup, and instance backup are the long SSH or dump calls that blocked a request. They now go through `background_jobs` (migration `00044`):

- Claim with `FOR UPDATE SKIP LOCKED` and a 12 minute lease (longer than the 10 minute run timeout, so a slow job is not started twice)
- Two workers, one second poll
- Three attempts, backoff `attempts² × 10s`
- Per-run timeout of 10 minutes
- Cancel only while the job is still `queued`
- Status at `GET /api/v1/jobs` and `dfin jobs`

Scheduled backups and cleanups still run from the scheduler and keep their own execution tables.

## Reviewed and left as they are

- SQL for the transfer uses a fixed list of table names, not request input.
- Scheduled task commands are shell-quoted. Server exec is limited to owners and admins.
- Container file paths go through `CleanPath`.
- Session cookies are `HttpOnly` and `SameSite=Lax`. CORS does not combine `*` with credentials. The dashboard sends a Content-Security-Policy. OAuth uses PKCE. WebAuthn’s relying party comes from `DOCKFIN_PUBLIC_URL`, which must now be absolute.
- Login is rate limited. Webhook HMAC is required unless `DOCKFIN_ALLOW_UNSIGNED_WEBHOOKS=1` (default off). The master key must be at least 32 characters, and the example key is refused outside development.
- Query-string webhook secrets (`?secret=`, GitLab `?token=`) stay accepted so existing GitLab hooks keep working. Request logs record `URL.Path`, not the raw query.
- OpenAPI `info.version` stays `dev`. The product version is `VERSION.md`. A test asserts the spec version.

## Dependency scan

`GOTOOLCHAIN=go1.26.10 govulncheck ./...` reports no vulnerabilities in code this module calls. One advisory remains in a required module that this code does not call.

Updated:

- `golang.org/x/crypto` v0.58.0, `golang.org/x/text` v0.43.0, `github.com/go-chi/chi/v5` v5.3.0, `github.com/jackc/pgx/v5` v5.9.2
- `go.mod` toolchain `go1.26.10`. The API image builds `FROM golang:1.26.10-alpine`. GitHub Actions already uses `go-version: 1.26.x`, which tracks the current 1.26 patch.
- Dashboard: Vite 8.3.6, Monaco 0.57.0, and a `dompurify` override at 3.4.17. `npm audit` in `apps/web` and `website` reports 0 vulnerabilities.

The standard library findings (`net/http`, `crypto/tls`, `crypto/x509`, and others) are fixed by Go 1.26.10. A binary built with an older 1.26.0 toolchain is still exposed; release builds must use 1.26.10 or newer.

`GO-2026-5932` is still recorded against `golang.org/x/crypto`. The fix is only in a commit after v0.58.0, and this code does not call the affected symbol. Dockfin stays on the latest release, v0.58.0.
