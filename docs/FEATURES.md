# Feature checklist (Coolify parity)

## Auth & teams
- [x] Register / login / logout / session cookies
- [x] Personal team on signup
- [x] Create shared (org) team — `POST /teams` + Team page UI
- [x] Team switcher API
- [x] Bearer token returned on login
- [x] Team invitations UI
- [x] API tokens with abilities UI
- [x] OAuth login runtime for enabled providers (`/auth/oauth/{provider}/start|callback`)
- [x] TOTP 2FA (setup/enable/disable + login challenge + recovery codes)
- [x] Forgot / reset password (SMTP/Resend when configured)

## Servers
- [x] SSH private keys (encrypted)
- [x] Servers CRUD
- [x] Validate (TCP + Docker + data dirs)
- [x] Destinations (default network)
- [x] Traefik proxy start/stop
- [x] Proxy dynamic Traefik file configs + proxy container logs
- [x] Sentinel agent install/restart/stop/logs + token rotate (SSH docker metrics agent)
- [x] Docker cleanup schedule (cron + threshold + force) with execution history
- [x] SSH host key TOFU / fingerprint persistence
- [x] Remote exec helper
- [x] Caddy proxy
- [x] Swarm destinations
- [x] Provision a VPS from a cloud token (Hetzner / DigitalOcean / Vultr) and auto-register it
- [x] Cloudflare Tunnel connector install/stop/status per server
- [x] Server log drain settings (newrelic / axiom / custom) written to `/data/dockfin/log-drain.env`
- [x] Custom CA certificate pushed to `/data/dockfin/ca/custom-ca.crt`
- [x] Terminal ACL (`terminal_acl_user_ids`) enforced on terminal create
- [x] Pending OS security patch check
- [x] Server health page: reachability, disk threshold, Traefik image age, and a manual check that emits the existing notification events
- [x] Traefik unknown-host catch-all (off by default; 503 or redirect)

## Projects
- [x] Projects + production environment
- [x] Extra environments API
- [x] Shared env vars API
- [x] Per-resource env vars CRUD + resolve at deploy

## Applications
- [x] CRUD + update + rollback
- [x] Build packs: dockerfile, compose, image, railpack, static
- [x] Deploy queue + SSE logs + cancel + queue limits
- [x] Start / Stop / Restart (single-container + compose)
- [x] Live container logs SSE (Logs tab)
- [x] Pre / post deployment commands
- [x] HTTP basic auth + custom Traefik/Caddy labels
- [x] Editable persistent volumes (non-compose)
- [x] Custom docker run / compose build+start / preserve repository
- [x] Preview environment variables (prod/preview tabs + deploy merge)
- [x] Deploy key selector on Git Source panel
- [x] Application Backups top tab (volume/dir → S3)
- [x] WWW redirect (`both` / `www` / `non-www`), including a permanent 301 option
- [x] Maintenance page without stopping the containers (Traefik file provider; Caddy stores the flag and warns)
- [x] Container file browser (list, read, write, mkdir, delete)
- [x] Advanced: disable build cache, shallow clone, Git LFS, GPU, stop timeout, restart policy
- [x] Additional destinations (multi-server fan-out via image transfer)
- [x] Per-container metrics (`docker stats`)
- [x] Compose multi-service terminal container picker
- [x] Private Docker registry credentials (`docker login` before pull)
- [x] Change build pack after create
- [x] Railpack/Static: install/build/start commands, publish dir, SPA, custom nginx
- [x] Ports mappings + custom network aliases
- [x] Healthcheck: CMD type, start period, scheme/host/response text
- [x] Preview URL template + manual preview deploy + public PR preview toggle
- [x] Clone application + Stop + Docker cleanup
- [x] App backup restore (untar volumes)
- [x] Rollback retention (`docker_images_to_keep`) + list server images
- [x] Advanced build: inject ARGs, SOURCE_COMMIT, build secrets, skip rebuild if unchanged
- [x] Consistent container name / custom internal name
- [x] Gzip / strip-prefix Traefik middlewares; GPU device IDs; max restart count
- [x] Empty Compose create (paste raw YAML, no Git)
- [x] Env sort + build-secret marking; logs download / timestamps / line limit
- [x] Swarm app settings: replicas, placement constraints, worker-only
- [x] Git webhook + HMAC verify
- [x] GitHub App webhook events (push + PR auto-deploy / preview / close cleanup)
- [x] Multi-provider webhooks (GitHub, GitLab MR, Gitea, Bitbucket) + `[skip ci]`/`[skip cd]`
- [x] PR preview create on webhook
- [x] PR/MR close auto-cleanup of preview deployments
- [x] Runtime env injection, limits, healthcheck wait
- [x] HTTPS Traefik labels
- [x] HTTPS Caddy labels (caddy-docker-proxy)
- [x] Dedicated build server split
- [x] Full GitHub App OAuth install UI

## Databases
- [x] Unified engines table (9 engines, including SQLite as a mounted file)
- [x] Remote start/stop over SSH
- [x] S3 storage + scheduled backup APIs
- [x] Backup dump helper
- [x] Full restore UI
- [x] Scheduled backup S3 upload
- [x] Multiple S3 destinations per scheduled backup
- [x] Patch API (`PATCH /databases/{id}`) + Configuration tab UI for `is_public`/`public_port` (requires restart/redeploy to take effect on the running container)
- [x] Import backup (upload a dump, stored under `/data/dockfin/backups`, optional immediate restore)
- [x] Logs tab (SSE stream from the `dockfin-db-{id}` container, mirrors application logs)
- [x] Terminal tab (reuses `ServerTerminal` against the database container)
- [x] Metrics tab (links to the destination server's host metrics)
- [x] Tags tab (`ResourceTagsPanel`, resourceType=database)
- [x] Files tab for the database container
- [x] Redis / KeyDB / Dragonfly flush (`FLUSHALL`)

## Services
- [x] Catalog loader (builtin + templates/compose)
- [x] Custom compose create
- [x] Remote `docker compose up` deploy

## Ops
- [x] Scheduled tasks API
- [x] Scheduled task + backup cron runner
- [x] Notification settings + deploy webhooks
- [x] Sentinel metrics ingest (token auth)
- [x] Terminal/exec helper
- [x] App detail + notifications UI
- [x] Full xterm browser terminal
- [x] Metrics charts UI polish
- [x] Application / database delete (danger zone)
- [x] Top-level Destinations nav (all destinations across servers, deep-link to server tab)
- [x] Top-level Tags nav (Coolify-style tag browser with attached-resource list)
- [x] Top-level Terminal nav (server picker + xterm, no detail page needed)
- [x] Environment clone success toast (application/database/service counts)
- [x] Settings → Scheduled Jobs "Recent issues" (failed task executions + failed docker cleanup runs)
- [x] Shared Variables server-scope hub (`/shared-variables?scope=server&server_id=...`, server picker, deep-link from Server → Settings)
- [x] Instance backup restore (Settings → Backup, confirm `RESTORE`)
- [x] Service volume backups (list/run/restore + scheduled)
- [x] Database dump/restore for MongoDB, ClickHouse, Dragonfly
- [x] SSH jump host / bastion (`jump_host_id` on servers)
- [x] Team audit log (`GET /audit-logs` + Audit nav)
- [x] Cloudflare DNS A-record upsert (`POST /domains/cloudflare`)
- [x] Log drain ships container logs via Vector when enabled

## Cloud provisioning & edge (wave 4)

`POST /api/v1/cloud-tokens/{tokenID}/provision` (alias `POST /api/v1/servers/provision`, both
admin-only) creates a VPS and registers it:

1. The stored provider token is decrypted and the selected private key's public key is uploaded to
   the provider if it is not already there.
2. An instance is created with cloud-init user data — either the selected cloud-init script or a
   built-in `#cloud-config` that installs Docker.
3. Dockfin polls for up to 90s for a public IPv4, then calls `CreateServer` with `root@IP:22` and
   the chosen private key.

Region/size/image are free-form provider identifiers. `GET /cloud-tokens/{id}/defaults` returns the
fallbacks used when they are blank (Hetzner `nbg1`/`cpx11`/`ubuntu-24.04`, DigitalOcean
`nyc3`/`s-1vcpu-1gb`/`ubuntu-24-04-x64`, Vultr `ewr`/`vc2-1c-1gb`/OS id `2284`). For Vultr a numeric
image is treated as `os_id`, anything else as a marketplace `image_id`. Run **Validate** on the new
server once cloud-init has finished installing Docker.

The server detail **Edge** tab covers the rest:

- **Cloudflare Tunnel** — `POST /servers/{id}/cloudflare-tunnel/{install|restart|stop|status}` runs
  `cloudflare/cloudflared tunnel run --token …` as the `dockfin-cloudflared` host-network container.
  The token is persisted (encrypted) in the server ops settings.
- **Log drain** — saving `log_drain_*` via `PATCH /servers/{id}/ops` also writes
  `/data/dockfin/log-drain.env` on the host (removed when disabled). Deploys are unaffected;
  shipping container logs is left to the drain agent.
- **CA certificate** — saving `ca_certificate` writes `/data/dockfin/ca/custom-ca.crt`.
- **Terminal access** — `terminal_acl_user_ids` is enforced by `POST /servers/{id}/terminal`. Empty
  list keeps the old behaviour; owners/admins always pass.
- **Security patches** — `POST /servers/{id}/patches/check` (admin-only) shells out to apt/dnf/yum/apk
  and returns the raw upgradable list.

Remote writes are best-effort: if SSH is unavailable the settings still save and the response
carries a `warnings` array.

## MCP & auto-update (wave 3)
- [x] HTTP JSON-RPC MCP endpoint (`POST /api/v1/mcp`, `GET` probe) gated on `is_mcp_server_enabled`
- [x] Bearer API-token auth (same path as the REST API — abilities and IP allowlist still apply)
- [x] Tools: `list_servers`, `list_projects`, `list_applications`, `get_application`, `deploy_application`, `stop_application`, `list_databases`, `list_services`, `deploy_service`
- [x] Scheduler auto-update tick (`is_auto_update_enabled` + `auto_update_frequency` cron)
- [x] Channel → tag mapping (stable→`latest`, next→`next`, nightly→`nightly`) on `ghcr.io/foisalislambd/dockfin`, honouring `docker_registry_url`
- [x] Rewrites the install compose image tag, then runs `docker compose pull` + `up -d` in `DOCKFIN_DIR`
- [x] Requires a mounted `/var/run/docker.sock` (or `DOCKFIN_AUTO_UPDATE=1`); install scripts now mount the socket and the install dir at `/host/dockfin`
- [x] Last run status persisted (`auto_update_last_at/status/message`, returned by `GET /settings`)

## Service logs, compose editor & DB metrics (wave 4)
- [x] `GET /services/{id}/logs/stream` (SSE, `tail` + `container` query params, accepts a bare compose unit name)
- [x] `GET /services/{id}/containers` (live containers, falls back to `dockfin-svc-{id8}-{unit}-1`)
- [x] Service detail Logs tab: live container logs with unit picker, tail control, download, reconnect
- [x] `PATCH /services/{id}` accepts `docker_compose_raw` — validated, stored, and prepared copy cleared so the next deploy re-prepares
- [x] Service detail General → editable compose editor with Save / Reset
- [x] `GET /databases/{id}/metrics` (docker stats for `dockfin-db-{uuid}`)
- [x] Database Metrics tab shows container stats above host metrics

## Secret managers
- [x] Team integration tokens for Doppler, Infisical, and HashiCorp Vault (token encrypted, never returned by the API)
- [x] One secret source per application, database, or service (`{{vault.KEY}}` references; values fetched only at deploy)
- [x] Key-name browser and import of missing references (invalid remote names are skipped, values are not stored)
- [x] Resolution on application runtime/build/compose env, service `.env`, and database container env

## File mounts
- [x] Applications and databases can mount a directory or a text file
- [x] File content is encrypted and written on the server before the container starts
- [x] A missing file is created first, so Docker does not turn the mount into a directory
- [x] Symbolic links are refused, and database mounts stay outside the engine data directory
- [x] Compose applications keep their volumes in the compose file

## GitHub Actions runners
- [x] A server can take organization workflow jobs for a custom label
- [x] Each job runs in its own container and is removed when the job ends
- [x] The host Docker socket is not mounted; Docker jobs use a private privileged daemon
- [x] Pull request jobs are refused unless the server allows them
- [x] Idle runners, long jobs, and jobs waiting for capacity are stopped

## Traffic analytics
- [x] A server can turn on Traefik JSON access logs; Caddy is refused
- [x] Sentinel sends new log lines, and the API stores minute totals for host, path, method, and status
- [x] Client addresses are stored in full, anonymized to a network, or not stored
- [x] A sample threshold keeps the first requests for a host in a batch and then one in ten
- [x] Totals older than the retention window are deleted, and an application or service sees only its own hosts

## Database init scripts
- [x] PostgreSQL, MySQL, and MariaDB accept `.sql` and `.sh` files; MongoDB accepts `.js` and `.sh`
- [x] Script bodies are encrypted and are not included in the database list
- [x] The next start writes them outside the data directory and mounts `/docker-entrypoint-initdb.d` read-only
- [x] The image runs them only while the data directory is empty, in filename order
- [x] Filenames cannot contain a path or shell metacharacters, and a mount cannot hide the init directory

## Database TLS
- [x] Per-database SSL switch for PostgreSQL, MySQL, MariaDB, MongoDB, Redis, KeyDB, and Dragonfly
- [x] A server CA is generated once and signs a certificate whose common name is the database id
- [x] Certificate and private key are encrypted; the API returns the CA certificate only
- [x] Start mounts the certificate and turns TLS on for that engine
- [x] Turning SSL off deletes the database certificate; the server CA is kept
- [x] Regenerate drops the leaf certificate so the next start signs a new one

## Managed DNS
- [x] Sync Cloudflare zones from an existing cloud token (`POST /cloud-tokens/{id}/dns-zones/sync`)
- [x] Saving an application, service, or preview hostname creates an A/AAAA record at the server's public address
- [x] Records Dockfin creates carry `managed-by: dockfin` and are updated only while that comment is still present
- [x] Existing records with the same address are referenced and never modified or deleted
- [x] A conflicting record is left unchanged and the save still succeeds, with a warning
- [x] A shared hostname is removed from Cloudflare only when the last resource releases it
- [x] Magic sslip.io / nip.io hostnames are not sent to Cloudflare

## Fleet pulse and deploy freeze
- [x] `GET /overview` — counts, 7-day deploy window, queue depth, recent deploys, and attention items (failed latest deploy, unhealthy status, failed backups)
- [x] Environment deploy freeze (`PUT /projects/{id}/environments/{envID}/freeze`) pauses dashboard, webhook, preview, MCP, and rollback deploys
- [x] Owners and admins can override a freeze on a single manual deploy; webhooks stay skipped
- [x] Home, environment, application, and service screens show the pause and what needs attention
- [x] Turning a freeze on notifies channels that opted into `deploy_freeze`
- [x] `dfin freeze` and `dfin unfreeze` pause or resume an environment; deploy and rollback accept `--override-freeze`
- [x] Deployment list responses omit log blobs; the deployment detail endpoint still returns them
- [x] Project and environment lists compute emptiness in one query
- [x] JSON panic responses, `nosniff` / frame / referrer headers, and version on `/health`
- [x] Global and per-route rate limits (auth, webhooks, sentinel) with `Retry-After`
- [x] Prometheus text at `/metrics` and structured request logs
- [x] OpenAPI 3.1 generated from the live router at `/openapi.json` and `/api/docs`, checked into `packages/openapi/openapi.yaml`
- [x] Audit log with filters, pagination, retention, and an owner/admin page
- [x] Content-Security-Policy compatible with the built dashboard and web terminal
- [x] Optional bearer token for `/metrics`
- [x] Postgres-backed shared rate limit (`DOCKFIN_RATE_LIMIT_STORE=postgres`), in-process by default
- [x] `dfin` CRUD for servers, projects, environments, apps, services, databases, env vars, and domains, with `--json` and `logs --follow`
- [x] API tokens can be paused, resumed, and rolled. Paused tokens fail authentication
- [x] Deployment emails can include the commit SHA and message
- [x] Settings can detect the instance public IPv4 and IPv6
- [x] `dfin tokens`, `maintenance`, `files`, `flush-cache`, and `server-health`

- [x] Image scans run Syft and Grype on the server that has the image, after a deploy and on demand (`POST /applications/{id}/image-scans`). Findings are stored and shown on Vulnerabilities. The instance gate is off, warn, or block.
- [x] A secret bundle exports the server private key, environment variables, shared variables, and database credentials, encrypted with a passphrase (`POST /servers/{id}/bundle`, `POST /server-bundles`). Import stores them under this instance's master key.
- [x] A Swarm manager starts Caddy or Traefik as a stack and runs the maintenance page as a Swarm service on the overlay network. The same Caddyfile holds maintenance and catch-all.
- [x] Same-instance server transfer moves the server and the resources that run on it. Type MOVE to confirm. A project that also has resources on another server stays; this server's resources are copied into a new project. Git sources, Docker registries, S3 backup targets, and GitHub runner configurations stay on the original team. An application that also runs on another server blocks the transfer. Metadata export still omits the private key; use the secret bundle to move secrets.
- [x] Newer image tags are chosen the way Coolify chooses them: same prefix and suffix, a patch on the same major.minor, a line on the same major, and a higher major for information only. Digests are recorded when the registry returns them. The tag is not treated as a fix until the next scan. Type UPGRADE to change a Docker image tag and deploy, or REBUILD to redeploy the current image. Both are audited.
- [ ] Hostinger purchase charges that account, can return a payment-pending order, and is rate limited at HTTP 429.
- [ ] Coolify canvas boards and the hosted subscription are not part of Dockfin. Canvas was excluded on purpose. The hosted subscription does not apply to this self-hosted install.

## Polish
- [x] Command palette (⌘K)
- [x] CLI `dfin` (deploy, rollback, freeze, logs, overview)
- [x] Production install.sh
- [x] OpenAPI document generated from the router
- [x] Docs + FEATURES
- [x] Embedded migrations
