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
