# Goolify

Open-source, self-hosted PaaS — a modern **Coolify alternative** built with **Go**, **React (Vite)**, and **PostgreSQL**.

Manage your own servers over SSH + Docker. No vendor lock-in: configs live on your hosts under `/data/goolify`.

## Status

Foundation + parity roadmap milestones M0–M6 implemented as a working control plane:

- Auth, teams, SSH host-key trust, servers, Traefik
- Applications (dockerfile / compose / image / nixpacks / static), deploy queue, SSE logs, cancel, rollback
- Env vars + shared vars, git webhooks (HMAC), PR preview hooks
- Databases (remote start/stop), S3/backup APIs, one-click services (+ Coolify template catalog loader)
- Onboarding wizard, app detail, notifications UI, `glfy` CLI, install script

See [docs/FEATURES.md](docs/FEATURES.md) for the detailed checklist.

## Quick start (development)

### Prerequisites

- Go 1.23+
- Node 22+
- PostgreSQL 16

### 1. Environment

```bash
cp .env.example .env
```

### 2. Database + API

```bash
docker compose -f deploy/compose/docker-compose.dev.yml up -d
go run ./cmd/goolify migrate
go run ./cmd/goolify serve
```

### 3. Web UI

```bash
cd apps/web
npm install
npm run dev
```

Open http://localhost:5173 — register, run Onboarding, deploy.

## Production install

```bash
sudo bash scripts/install.sh
```

## Architecture

```
React SPA  →  Go API (chi) + workers  →  PostgreSQL
                      │
                      └── SSH ──→ remote Docker / Traefik
```

## License

Apache-2.0 — see [LICENSE](LICENSE).

## Reference

The `coolify/` directory is an upstream reference checkout (templates reused as catalog source) and is not required at runtime if you ship your own `templates/compose`.
