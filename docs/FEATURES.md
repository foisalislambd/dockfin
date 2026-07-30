# Feature checklist (Coolify parity)

## Auth & teams
- [x] Register / login / logout / session cookies
- [x] Personal team on signup
- [x] Team switcher API
- [x] Bearer token returned on login
- [x] Team invitations UI
- [x] API tokens with abilities UI

## Servers
- [x] SSH private keys (encrypted)
- [x] Servers CRUD
- [x] Validate (TCP + Docker + data dirs)
- [x] Destinations (default network)
- [x] Traefik proxy start/stop
- [x] SSH host key TOFU / fingerprint persistence
- [x] Remote exec helper
- [ ] Caddy proxy
- [ ] Swarm destinations

## Projects
- [x] Projects + production environment
- [x] Extra environments API
- [x] Shared env vars API
- [x] Per-resource env vars CRUD + resolve at deploy

## Applications
- [x] CRUD + update + rollback
- [x] Build packs: dockerfile, compose, image, nixpacks, static
- [x] Deploy queue + SSE logs + cancel + queue limits
- [x] Git webhook + HMAC verify
- [x] PR preview create on webhook
- [x] Runtime env injection, limits, healthcheck wait
- [x] HTTPS Traefik labels
- [ ] Dedicated build server split
- [ ] Full GitHub App OAuth install UI

## Databases
- [x] Unified engines table (8 engines)
- [x] Remote start/stop over SSH
- [x] S3 storage + scheduled backup APIs
- [x] Backup dump helper
- [x] Full restore UI

## Services
- [x] Catalog loader (builtin + coolify/templates/compose)
- [x] Custom compose create
- [x] Remote `docker compose up` deploy

## Ops
- [x] Scheduled tasks API
- [x] Notification settings + deploy webhooks
- [x] Sentinel metrics ingest (token auth)
- [x] Terminal/exec helper
- [x] Onboarding + app detail + notifications UI
- [ ] Full xterm browser terminal
- [x] Metrics charts UI polish

## Polish
- [x] Command palette (⌘K)
- [x] CLI `glfy` (deploy/logs/apps/servers)
- [x] Production install.sh
- [x] OpenAPI skeleton
- [x] Docs + FEATURES
- [x] Embedded migrations
