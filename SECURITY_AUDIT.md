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
- Query-string webhook secrets (`?secret=`) stay accepted so existing hooks keep working. GitLab system hooks now accept only the `X-Gitlab-Token` header; a `?token=` query value is ignored so it cannot land in access logs. Request logs record `URL.Path`, not the raw query.
- OpenAPI `info.version` stays `dev`. The product version is `VERSION.md`. A test asserts the spec version.

## 2026-11-25 — tests that found defects

| Area | What was wrong | Fix |
| --- | --- | --- |
| Crashed job | `RequeueExpiredJobs` marked a last-attempt job failed and stopped. A deletion kept an empty `delete_error`, so the dashboard showed it as still deleting and hid Retry. A start/stop/restart stayed on `stopping`, `starting`, or `restarting`. | The same update writes `delete_error` on the tombstone, or sets the resource status to `error` for a power job. |
| Domain field | `NormalizeDomains` turned `javascript:alert(1)`, `not a host`, and `0/..` into values that could be stored and emitted into proxy config or links. `isValidHostnameList` accepted the host label `javascript` because the port was discarded before the check. | Normalization rebuilds `scheme://host[:port][/path]` and drops whitespace, quotes, non-numeric ports, and `..` segments. The API rejects an entry the normalizer drops. Fuzz seed `0/..` is kept under `internal/proxy/testdata`. |

Reviewed in the new matrix and left as they are: unauthenticated API calls return 401, a member limited to one project gets 404 for another project’s application and env vars, a second registration is refused, the session cookie is `HttpOnly` and `SameSite=Lax` without `Secure` on plain HTTP, CORS does not echo an unknown origin, and a project name containing SQL text is stored as data.

## Dependency scan

`govulncheck ./...` with Go 1.26.11 reports no vulnerabilities in code this module calls. One advisory remains in a required module that this code does not call.

Updated:

- `golang.org/x/crypto` v0.58.0, `golang.org/x/text` v0.43.0, `github.com/go-chi/chi/v5` v5.3.0, `github.com/jackc/pgx/v5` v5.9.2
- `go.mod` toolchain `go1.26.11`. The API image builds `FROM golang:1.26.11-alpine`. GitHub Actions already uses `go-version: 1.26.x`, which tracks the current 1.26 patch.
- Dashboard: Vite 8.3.6, Monaco 0.57.0, and a `dompurify` override at 3.4.17. `npm audit` in `apps/web` and `website` reports 0 vulnerabilities.

The standard library findings (`net/http`, `crypto/tls`, `crypto/x509`, and others) are fixed by Go 1.26.11, including `GO-2026-6859` (HTTP/1 `MaxBytesReader` header race, CVE-2026-97054). Dockfin reads JSON bodies through `MaxBytesReader`. A binary built with 1.26.10 or older is still exposed; release builds must use 1.26.11 or newer.

`GO-2026-5932` is still recorded against `golang.org/x/crypto`. The fix is only in a commit after v0.58.0, and this code does not call the affected symbol. Dockfin stays on the latest release, v0.58.0.

## 2026-11-25 — second pass

| Area | What was wrong | Fix |
| --- | --- | --- |
| Overview | A member limited to one project still received every project, deployment, and the hidden application id from `GET /overview`. | `redactOverview` filters those lists and resource counts. Deploy-window and queue totals are zeroed for a restricted member. |
| Deploy freeze | Unfreeze sent an untyped actor, and Postgres rejected `deploy_frozen_by` (`42804`, uuid versus text). The handler returned 500. | The SQL casts `$6::uuid` and the store always passes a `uuid.UUID` (`uuid.Nil` when unfreezing). |
| Server import | An incomplete bundle reached `ImportServer`, which returned a plain error and became a 500. | The handler returns 400 `server bundle is incomplete` before the store call. |
| S3 and Hostinger | A storage endpoint or a custom Hostinger base URL was requested as given, including metadata addresses. | `netsafe.ValidateHTTPURL` blocks metadata, link-local, and (unless private is allowed) loopback and RFC1918. Hostinger additionally allows only `developers.hostinger.com`, `127.0.0.1`, and `localhost`. |
| Runner docker args | `StartArgs` appended an unvalidated image, so `--privileged` became a docker flag. A hook path could contain `..`, a colon, or a NUL and be spliced into `-v`. | Image, CPU, memory, and absolute paths are validated. `:` and NUL are rejected so they cannot add another mount. |
| Env files | `FormatEnvFile` wrote a raw key, so a newline in the key injected another assignment. A quoted value with a backslash was unescaped in the wrong order, turning `\\n` into a newline. | Invalid keys are skipped. Quoted values escape `\` before `"`. Parsing walks escapes left to right. |
| Database and backup paths | An id or filename containing `..` or a separator could leave `/data/dockfin`. | `HostDataDir`, `DumpPath`, and backup retention globs fall back to `invalid` and reject `..`. |
| Git remotes | An scp-style remote could contain `;` or `&`. `ValidateRemote` also treated a trimmed shorthand as safe while the raw string still had leading space; that was a test bug. IPv4-mapped metadata addresses were not classified as link-local. Shorthand `file:/0` and `169.254.169.254/0` were accepted, and `http://0169.254.169.254` was accepted because `net.ParseIP` rejects a leading zero. | Shell metacharacters are rejected. Shorthand must be a single `owner/name` with a letter in each part, no colon, and no metadata address. `ParseIPLoose` reads dotted IPv4 with leading zeros so `0169.254.169.254` is the link-local metadata address. |
| GitLab system hook | A body of `null` unmarshals to a nil map and panicked in `NormalizeGitLabSystemHook`. | A nil object is an invalid hook. |
| GitHub webhook | Signature comparison is HMAC-SHA256. A 90 second fuzz run found no accepted bad signature. GitLab application hooks compare `X-Gitlab-Token` in constant time; they are not HMAC. | No code change. |
| Application clone | `copyResourceExtras` dropped `IsPreview`, so a preview application cloned as a normal one. | The copy sets `IsPreview`. |
| `dfin apps create --build-pack` | The flag was converted to `build_pack` and then deleted from the JSON body, so the server never received the pack. | The delete was removed. |
| Project dialog | Description used the default required input, so Create did nothing when the description was empty. | Description is optional. |
| Repository parser | `ParseRepo` replaced the first colon, so `https://github.com/acme/web` was parsed as host `https`. `git@` URLs only worked by accident. | `git@host:path` is rewritten to an ssh URL. HTTPS URLs are parsed as written. Metadata hosts and `..` path segments are rejected. |
| Environment keys | `POST /env-vars` stored a key containing a newline, which can become a second assignment when the file is rendered. | Keys must match `ValidEnvKey` (`[A-Za-z_][A-Za-z0-9_.]*`). |
| Application domain | `isValidHostnameList` accepted `http://169.254.169.254/latest`, and the API stored it and returned it as a link. | The host is rejected when `BlockedFetchHost` matches. |
| Traefik host rule | A domain with a space still produced a Host rule for `not a host`. | Rules skip hosts that are not a hostname. A leading `*.` wildcard is allowed. |
| Volume dump path | `dumpContainerDirCmd` accepted an output path containing `..`. | The command builder returns an error for `..` in the path or container name. |
| App backup filename | `DefaultAppBackupFilename` sliced the first 8 characters of a short id and panicked. | Shorter ids are used whole. |

### Scanner exclusions

CI runs `staticcheck` (SA* and the checks not listed below), `gosec` with `.gosec.json`, `govulncheck`, and `npm audit --audit-level=high`.

staticcheck ignores, each one existing style or unfinished call sites rather than a defect in a called path:

- `U1000` unused funcs kept for deploy and DNS parity: `pruneAppImages`, `waitStartPeriod`, `swarmStackOverrides`, `zoneFromHost`, `removeResourceScopedNetwork`, `scanOnServer`, `dialServerForTeam`, `openAPIRoute`
- `ST1000`, `ST1005` comment style
- `ST1003` initialisms. Exported names use `Api` (`ApiToken`) to match `api_tokens`. Renaming them to `API` would be a Go-only break
- `S1009`, `S1011`, `S1021` nil-len and declaration style
- `SA1019` `chimw.RealIP` is only installed when `DOCKFIN_TRUST_PROXY` is set, so a client cannot supply the forwarded address

gosec rules excluded, with the reason:

| Rule | Why it stays |
| --- | --- |
| G101 | Field names and the default runner image, not embedded secrets |
| G104 | `Response.Body.Close` errors after the body is already consumed |
| G115 | Integer conversions on counters and file sizes that are range-checked by the caller |
| G118 | Goroutines that intentionally outlive the request (scheduler, runner provision) |
| G124 | OAuth cookies are `SameSite=Lax`. `Secure` follows the panel URL; tests and plain HTTP installs do not set it |
| G204 | `pg_dump`, `pg_restore`, and `docker` use argument vectors, not a shell string |
| G304 / G703 | Template and compose paths are under the configured template directory or the service data dir |
| G306 | The bootstrap public key is `0644`. The private key is `0600` |
| G702 | `git clone` builds an argument vector. gosec flags the `...` expansion |
| G704 | Outbound URLs go through `netsafe.ValidateHTTPURL` or the Hostinger host allowlist |
| G710 | Request bodies are already limited by `maxJSONBody` |
