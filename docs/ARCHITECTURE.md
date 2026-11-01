# Architecture

## Overview

Dockfin is a **central control plane** that manages remote Docker hosts over **SSH**. There is no long-lived deploy agent on target machines. Desired state lives in PostgreSQL; workers execute builds and lifecycle actions on servers via SSH.

```
Browser (React) ──REST/SSE──► dockfin serve (Go)
                                  │
                                  ├── PostgreSQL (state, deployments, secrets)
                                  ├── in-process deploy queue (workers)
                                  └── SSH ──► remote Docker / Traefik
```

## HTTP surface

The Chi router is the source of truth for the API. `Router()` walks itself and writes an OpenAPI 3.1 document served at `/openapi.json` (also `/api/v1/openapi.json`). `packages/openapi/openapi.yaml` is that document at the default rate-limit policy, with `info.version` kept at `dev` so release ldflags do not churn the file.

Rate limits are sliding windows keyed by client IP. The global bucket is 600 requests/minute. Auth routes are 30 per 15 minutes, webhooks and `/api/v1/deploy` are 120 per minute, and sentinel ingest is 600 per minute. `/health`, `/metrics`, CORS preflight, and static assets are exempt. Zero in config disables that bucket. The failed-login throttle still applies on top.

`GET /metrics` is Prometheus text. Series use the Chi route pattern, so UUIDs do not become labels. Request logs are structured and skip `/health` and `/metrics`.

## Domain model

```
Team
 ├── members (owner | admin | member)
 ├── private_keys
 ├── servers → destinations (Docker networks)
 └── projects → environments → applications | databases | services
```

Resources are always scoped by `team_id` for simple authorization filters.

## Deploy pipeline

Stages (conceptually):

1. **prepare** — SSH dial, host key, data dirs, Docker network  
2. **fetch** — git clone or image pull  
3. **build** — `docker build`, compose build, railpack, or static Dockerfile  
4. **run** — replace container, Traefik labels, env injection, limits  
5. **health** — optional wait until container running / healthy  
6. **finalize** — status, notifications  

Logs stream to the UI via SSE (`/api/v1/deployments/{id}/logs/stream`).

## Secrets

- Master key: `DOCKFIN_MASTER_KEY` (AES-256-GCM envelope)
- Passwords: argon2id
- SSH private keys and env values stored encrypted in Postgres
- Webhook secrets optional; when set, signatures are required

## Frontend

`apps/web` is a Vite + React SPA talking to the API with cookie sessions (`credentials: include`) and optional bearer tokens.

## Related Coolify concepts

Dockfin mirrors Coolify’s operator mental model (SSH hosts, Traefik, build packs, one-click services) but reimplements the control plane in Go with an API-first SPA.
