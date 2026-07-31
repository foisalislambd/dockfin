# Goolify

**An open-source, self-hosted PaaS** — deploy applications, databases, and one-click services on your own servers over SSH + Docker.

Built with **Go**, **React (Vite)**, and **PostgreSQL**. Fully open source under [MIT](LICENSE). No cloud paywall. No Stripe. No vendor lock-in.

> Configurations are written to your servers under `/data/goolify/…`. If you stop using Goolify, your containers and compose files remain manageable without the control plane.

---

## Why Goolify exists

[Coolify](https://coolify.io) proved that a self-hosted Heroku/Vercel alternative is possible. Goolify is a **from-scratch rewrite** aimed at the same job with a modern stack and cleaner architecture:

| Goal | Approach |
|------|----------|
| Faster, typed control plane | Go API + workers instead of a large PHP monolith |
| Better dashboard UX | React SPA (deep links, command palette, optimistic UI) |
| Simpler data model | Unified `databases` / `destinations` tables, `team_id` everywhere |
| Safer remote execution | Argv-style SSH commands, host-key trust (TOFU), encrypted secrets |
| True open source | MIT; no paid-only features |

Goolify is inspired by Coolify’s product model (`Team → Project → Environment → resources`, SSH agentless Docker hosts, Traefik proxy). It is **not** a PHP/Livewire port.

---

## Features

- **Auth & teams** — register/login, personal team, session cookies + bearer tokens
- **Servers** — SSH keys, validate Docker, Traefik proxy start/stop, host-key fingerprinting
- **Applications** — dockerfile, dockercompose, dockerimage, nixpacks, static
- **Deployments** — queue, cancel, rollback, live SSE logs, concurrency limits
- **Env vars** — per-resource + shared (`{{team.KEY}}`, `{{project.KEY}}`, …)
- **Git webhooks** — GitHub/GitLab HMAC verification, PR preview hooks
- **Databases** — PostgreSQL, MySQL, MariaDB, MongoDB, Redis, KeyDB, Dragonfly, ClickHouse
- **Services** — custom compose + **~360** one-click templates in `templates/compose/` (Coolify-compatible)
- **Ops** — notifications (Discord/Slack/webhook), scheduled tasks/backups APIs, sentinel metrics ingest
- **DX** — `glfy` CLI, one-click VPS smoke test, Docker Compose for local DB

See the full checklist: [docs/FEATURES.md](docs/FEATURES.md)

---

## Architecture

```
┌─────────────────────────────────────────┐
│  Goolify control plane                  │
│  React SPA  →  Go API + workers         │
│                 PostgreSQL (+ Redis)    │
└───────────────────┬─────────────────────┘
                    │ SSH
                    ▼
┌─────────────────────────────────────────┐
│  Your server(s)                         │
│  Docker Engine · Traefik · apps/DBs     │
│  /data/goolify/…                        │
└─────────────────────────────────────────┘
```

---

## Quick start (development)

### Prerequisites

- Go 1.26+
- Node.js 22+
- Docker (for Postgres/Redis) or a local PostgreSQL 16

### 1. Clone and configure

```bash
git clone https://github.com/YOUR_ORG/goolify.git
cd goolify
cp .env.example .env
# Edit GOOLIFY_MASTER_KEY and GOOLIFY_SESSION_SECRET (32+ characters each)
```

### 2. Start database

```bash
docker compose -f deploy/compose/docker-compose.dev.yml up -d
```

### 3. Run API

```bash
go run ./cmd/goolify migrate
go run ./cmd/goolify serve
```

API: http://localhost:8080/health

### 4. Run web UI

```bash
cd apps/web
npm install
npm run dev
```

UI: http://localhost:5173 — register an account, add a server under **Servers**, create a **Project**, deploy.

---

## Production install

```bash
sudo bash scripts/install.sh
```

This prepares `/data/goolify`, generates secrets, and starts a Compose stack.  
Until official images are published to a registry, prefer building from source on the VPS (see smoke test below).

---

## One-click VPS smoke test

On a fresh Ubuntu/Debian VPS (as root), from a cloned repo:

```bash
sudo bash scripts/vps-oneclick-test.sh
```

This installs dependencies, builds the API, registers a user, adds the VPS as a self-SSH server, starts Traefik, deploys `nginx:alpine`, and writes a report to `/opt/goolify-smoke/report.txt`.

Full guide: [docs/VPS-SMOKE-TEST.md](docs/VPS-SMOKE-TEST.md)

---

## CLI

```bash
export GOOLIFY_URL=http://localhost:8080
export GOOLIFY_TOKEN=<token-from-login-response>

glfy version
glfy health
glfy servers
glfy apps
glfy deploy <application-uuid>
glfy logs <deployment-uuid>
```

---

## Documentation

| Doc | Description |
|-----|-------------|
| [docs/README.md](docs/README.md) | Install, first deploy, webhooks, env vars |
| [docs/FEATURES.md](docs/FEATURES.md) | Feature parity checklist |
| [docs/VPS-SMOKE-TEST.md](docs/VPS-SMOKE-TEST.md) | Automated VPS smoke test |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | System design overview |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | Community standards |
| [CHANGELOG.md](CHANGELOG.md) | Release history |
| [SUPPORT.md](SUPPORT.md) | Where to get help |

OpenAPI sketch: [packages/openapi/openapi.yaml](packages/openapi/openapi.yaml)

---

## Project layout

```text
goolify/
  cmd/goolify/          # API binary (serve, migrate, version)
  cmd/glfy/             # CLI
  cmd/goolify-sentinel/ # Metrics agent stub
  internal/             # Go packages (httpapi, deploy, store, sshx, …)
  apps/web/             # React + Vite dashboard
  migrations/           # Goose SQL migrations (embedded via embed.go)
  deploy/               # Dockerfiles + Compose
  scripts/              # install.sh, vps-oneclick-test.sh
  packages/openapi/     # API contract
  docs/                 # Documentation
  coolify/              # Optional upstream reference (gitignored locally)
```

---

## Roadmap / status

Goolify is under active development. Core control-plane paths work; some Coolify-parity items remain (team invites UI, GitHub App OAuth UI, full xterm terminal, backup restore UI, etc.). Track them in [docs/FEATURES.md](docs/FEATURES.md).

**Not planned:** paid cloud billing, feature paywalls, Kubernetes in v1.

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and follow the [Code of Conduct](CODE_OF_CONDUCT.md).

```bash
go test ./...
cd apps/web && npm run build
```

---

## Security

Do not open public issues for sensitive vulnerabilities. See [SECURITY.md](SECURITY.md).

---

## License

Licensed under the [MIT License](LICENSE).

Coolify is a separate project with its own license and trademark. Goolify is an independent open-source alternative inspired by the same problem space.
