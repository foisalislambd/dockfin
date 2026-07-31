# AGENTS.md — Goolify development guide

Practical guide for humans and AI agents working on this repo. Prefer these commands over inventing new workflows.

## Layout (what matters day-to-day)

| Path | Role |
|------|------|
| `cmd/goolify` | API binary (`serve`, `migrate`, `version`) |
| `internal/` | Go packages (httpapi, services, deploy, proxy, store, …) |
| `apps/web/` | React + Vite dashboard |
| `templates/compose/` | One-click service YAML catalog |
| `migrations/` | Goose SQL (run via `goolify migrate`) |
| `deploy/docker/Dockerfile.api` | Single production image (API + baked Vite UI) |
| `deploy/compose/docker-compose.dev.yml` | Local Postgres + Redis (hot-reload mode) |
| `scripts/install.sh` | **Production** install — pulls `ghcr.io/foisalislambd/goolify` |
| `scripts/install-dev.sh` | **Dev** install — builds `goolify:local`, no registry pull |
| `.env` | Runtime config for hot-reload mode (copy from `.env.example`) |

Control-plane data:

| Mode | Where |
|------|-------|
| Hot reload (`go run`) | `GOOLIFY_DATA_DIR` (default `./data`) |
| Dev Docker (`install-dev.sh`) | Compose project `/data/goolify` + volumes `goolify-pg` / `goolify-data` |
| Production (`install.sh`) | Same dir/volumes; image switched to GHCR |

**Important:** App/DB/service files on a *target server* are always under host path **`/data/goolify/{applications,databases,services,proxy,backups}`** (hardcoded in Go over SSH). That is separate from the control-plane container’s `/data` volume.

---

## Install scripts (pick one)

| Script | When | Image | Dir |
|--------|------|-------|-----|
| `scripts/install.sh` | Real VPS / production | `ghcr.io/foisalislambd/goolify:latest` (pull) | `/data/goolify` |
| `scripts/install-dev.sh` | Agent/smoke testing full stack in Docker | `goolify:local` (build from this repo) | `/data/goolify` (same) |

Both scripts share `/data/goolify` and the same Docker volumes so switching prod ↔ local keeps the DB. Do **not** run two stacks on port 8000 at once.

### Production (GHCR)

```bash
curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash

# Pin a version
sudo GOOLIFY_VERSION=1.0.9 bash -c 'curl -fsSL …/install.sh | bash'

# Update later
cd /data/goolify && sudo docker compose pull && sudo docker compose up -d
```

Dashboard: `http://SERVER_IP:8000/` (ports 80/443 left free for Traefik).

### Development Docker stack (preferred for “run the whole product” on a VPS)

From the **repo root** (needs Docker + root for `/data`):

```bash
cd /root/goolify   # or your clone path
sudo bash scripts/install-dev.sh
```

What it does:

1. `docker build -f deploy/docker/Dockerfile.api -t goolify:local .`
2. Writes `/data/goolify/docker-compose.yml` + `.env` (`GOOLIFY_ENV=development`)
3. `docker compose up -d --pull never --force-recreate` (Postgres + Goolify on **:8000**)

```bash
# Health / version
curl -s http://127.0.0.1:8000/health
curl -s http://127.0.0.1:8000/api/v1/version

# Logs / restart
cd /data/goolify && docker compose logs -f goolify
cd /data/goolify && docker compose restart goolify
```

**After Go or UI code changes**, rebuild + recreate (same script is fine):

```bash
cd /root/goolify && sudo bash scripts/install-dev.sh
```

Or manually:

```bash
cd /root/goolify
docker build -f deploy/docker/Dockerfile.api --build-arg VERSION=dev -t goolify:local .
cd /data/goolify
unset GOOLIFY_DATABASE_URL GOOLIFY_MASTER_KEY GOOLIFY_HTTP_ADDR GOOLIFY_PUBLIC_URL 2>/dev/null || true
docker compose up -d --pull never --force-recreate goolify
```

Do **not** export host `GOOLIFY_DATABASE_URL=…@127.0.0.1` when using compose — it overrides the container DB host (`postgres`). Secrets stay in `/data/goolify/.env` only.

Optional overrides for `install-dev.sh`:

| Env | Default |
|-----|---------|
| `GOOLIFY_DIR` | `/data/goolify` |
| `GOOLIFY_IMAGE` | `goolify:local` |
| `GOOLIFY_VERSION` | `dev` (build-arg / version string) |
| `GOOLIFY_HOST_PORT` | `8000` |

---

## Prerequisites (hot-reload mode)

- Go **1.26+**
- Node.js **22+**
- Docker (for Postgres/Redis and deploying containers)

```bash
cp .env.example .env
# Set GOOLIFY_MASTER_KEY to ≥32 characters
```

---

## A) Everyday local development (hot reload UI)

Use this when iterating on API/UI quickly. For a full Docker control plane like production, use **`install-dev.sh`** instead.

### 1. Database

```bash
docker compose -f deploy/compose/docker-compose.dev.yml up -d
```

### 2. Migrate + API

```bash
set -a && source .env && set +a
go run ./cmd/goolify migrate
go run ./cmd/goolify serve
# or: make migrate && make api
```

API: `http://127.0.0.1:8000` · health: `/health` · version: `/api/v1/version`

Optional for templates + static UI when not using Vite:

```bash
export GOOLIFY_TEMPLATES_DIR="$PWD/templates/compose"
export GOOLIFY_WEB_DIR="$PWD/apps/web/dist"
```

### 3. Web (Vite)

```bash
cd apps/web
npm install   # first time
npm run dev   # http://localhost:5173 — proxies/CORS via GOOLIFY_CORS_ORIGINS
```

Use this mode when iterating on UI. API changes still need restarting `go run` / the binary.

---

## B) Rebuild + restart (binary + static UI)

Use when serving the built SPA from the API (`GOOLIFY_WEB_DIR`), e.g. VPS smoke at `/opt/goolify-smoke`.

For the **Docker dev stack**, prefer `sudo bash scripts/install-dev.sh` (section above) over this binary path.

```bash
# From repo root
go build -o /opt/goolify-smoke/bin/goolify ./cmd/goolify   # or: ./bin/goolify
cd apps/web && npm run build && cd ../..

# Stop old process
kill "$(cat /opt/goolify-smoke/api.pid 2>/dev/null)" 2>/dev/null || \
  pkill -f './bin/goolify serve' || true
sleep 1

# Start (adjust paths to your smoke/workdir)
cd /opt/goolify-smoke
set -a && source /root/goolify/.env && set +a
export GOOLIFY_TEMPLATES_DIR=/root/goolify/templates/compose
export GOOLIFY_WEB_DIR=/root/goolify/apps/web/dist
nohup ./bin/goolify serve >>api.log 2>&1 &
echo $! > api.pid
curl -s http://127.0.0.1:8000/api/v1/version
```

**Go-only change (no UI):** rebuild binary + restart serve — skip `npm run build`.  
**UI-only change:** `npm run build` is enough if `GOOLIFY_WEB_DIR` points at `apps/web/dist` (restart API only if it caches aggressively; usually a hard refresh is enough).

### Quick test packages

```bash
go test ./internal/services/ ./internal/httpapi/ ./internal/proxy/ -count=1
go test ./...   # full suite (slower)
cd apps/web && npm run build   # typecheck + Vite build
```

---

## C) Clean data / reset environments

Destructive — only on local or smoke hosts.

### Reset Docker stack (dev or production)

Same directory/volumes — wipe once, then reinstall with the script you want:

```bash
cd /data/goolify
docker compose down -v          # wipes goolify-pg + goolify-data

# Dev (local build):
cd /root/goolify && sudo bash scripts/install-dev.sh

# Or production (GHCR pull):
curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash
```

### Reset control-plane database (hot-reload Postgres volume)

```bash
# Stop API first
pkill -f 'goolify serve' || true

docker compose -f deploy/compose/docker-compose.dev.yml down -v
docker compose -f deploy/compose/docker-compose.dev.yml up -d
# wait until healthy
docker compose -f deploy/compose/docker-compose.dev.yml exec -T postgres pg_isready -U goolify

set -a && source .env && set +a
go run ./cmd/goolify migrate   # or: /opt/goolify-smoke/bin/goolify migrate
# then start serve again and register a fresh user
```

`down -v` deletes the `goolify-pg-dev` volume (all projects, users, env vars, etc.).

### Wipe local API data dir

```bash
rm -rf "${GOOLIFY_DATA_DIR:-./data}"/*
# smoke example:
rm -rf /opt/goolify-smoke/data/*
```

### Clean deployed Docker stacks on the host (smoke VPS)

Containers are named like `goolify-svc-<id8>-…` with project `goolify-svc-<id8>`:

```bash
# List
docker ps -a --filter name=goolify-svc

# Tear down one service project (id prefix = first 8 of service UUID)
docker compose -p goolify-svc-<id8> -f /data/goolify/services/<uuid>/docker-compose.yml down -v

# Nuclear: all goolify service containers + Traefik proxy (careful)
docker ps -aq --filter name=goolify-svc | xargs -r docker rm -f
docker rm -f goolify-proxy 2>/dev/null || true
# Optional: remove remote compose dirs
# rm -rf /data/goolify/services/*
```

Shared Docker network is usually `goolify`. Do not delete it while Traefik / stacks still need it.

### Soft “force re-prepare compose” (no full DB wipe)

Clears stored prepared YAML so the next deploy re-runs `PrepareCompose` (networks, magic env, Traefik labels):

```bash
docker exec compose-postgres-1 psql -U goolify -d goolify \
  -c "UPDATE services SET docker_compose='' WHERE id='<service-uuid>';"
# then POST /api/v1/services/<id>/deploy
```

(Container name may be `compose-postgres-1` or similar from `docker-compose.dev.yml`.)

---

## D) Env vars agents should know

| Variable | Purpose |
|----------|---------|
| `GOOLIFY_HTTP_ADDR` | Bind address (default `:8000`) |
| `GOOLIFY_DATABASE_URL` | Postgres DSN |
| `GOOLIFY_MASTER_KEY` | ≥32 chars; encrypts secrets |
| `GOOLIFY_PUBLIC_URL` | Panel public URL (cookies / links) |
| `GOOLIFY_PUBLIC_IP` | Magic domain base (`*.IP.sslip.io`) |
| `GOOLIFY_WEB_DIR` | Static SPA directory |
| `GOOLIFY_TEMPLATES_DIR` | One-click templates root |
| `GOOLIFY_DATA_DIR` | Control-plane local data |
| `GOOLIFY_CORS_ORIGINS` | Include Vite origin in dual-process mode |
| `GOOLIFY_COOKIE_SECURE` | `0` for plain HTTP IPs |
| `GOOLIFY_BOOTSTRAP_SELF` | Auto-register this host as a server |
| `GOOLIFY_IMAGE` | Override image for install scripts |
| `GOOLIFY_VERSION` | Tag / build-arg for install scripts |
| `GOOLIFY_DIR` | Install directory override |

Magic domains (`sslip.io` / `nip.io`) stay **HTTP** by default (no Let's Encrypt) — rate limits on shared magic DNS.

---

## E) Conventions (keep diffs small)

- Do **not** commit unless the user asks.
- Do **not** put service public domains in instance `.env`; they belong in **project Environment Variables** (`SERVICE_URL_*` / `SERVICE_FQDN_*`).
- Prefer matching existing patterns in `internal/services/compose.go` and `internal/httpapi/` over new abstractions.
- One-click templates: Coolify-style metadata (`# name:`, `# port:`, `# logo:`) + magic env keys.
- After Go or UI fixes on the **Docker dev** host: `sudo bash scripts/install-dev.sh`, then hard-refresh the browser.
- After Go or UI fixes on the **binary smoke** host: rebuild + restart (section B), then hard-refresh.
- Prefer `install-dev.sh` over inventing ad-hoc `docker compose` files for the control plane.
- Production image source of truth: **GHCR** `ghcr.io/foisalislambd/goolify` (also mirrored to Docker Hub).

---

## F) Useful one-liners

```bash
# Is API up?
curl -s http://127.0.0.1:8000/api/v1/version

# Docker control-plane stack
cd /data/goolify && docker compose ps
cd /data/goolify && docker compose logs -f goolify

# Rebuild + redeploy local image
cd /root/goolify && sudo bash scripts/install-dev.sh

# Find host binary serve PID
ps -eo pid,cmd | grep '[g]oolify serve'

# Tail binary smoke logs
tail -f /opt/goolify-smoke/api.log

# Full VPS smoke bootstrap (destructive / heavy)
sudo bash scripts/vps-oneclick-test.sh
```

More detail: [README.md](README.md), [docs/VPS-SMOKE-TEST.md](docs/VPS-SMOKE-TEST.md), [CONTRIBUTING.md](CONTRIBUTING.md).
