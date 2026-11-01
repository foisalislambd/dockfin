<p align="center">
  <img src="apps/web/public/brand/dockfin-logo.png" alt="Dockfin" width="180" />
</p>

# Dockfin

**Deploy apps on your own server** — like Heroku / Vercel, but self-hosted.

One Docker image = API + dashboard. MIT license. Your data stays on your VPS.

| Registry | Image |
|----------|--------|
| **GitHub Packages (GHCR)** — production default | `ghcr.io/foisalislambd/dockfin:latest` |
| [Docker Hub](https://hub.docker.com/r/foisalislambd/dockfin) | `foisalislambd/dockfin:latest` |

---

## Install on a production server

You need a **Ubuntu/Debian VPS** with a public IP. That’s it.

### One command (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/foisalislambd/dockfin/main/scripts/install.sh | sudo bash
```

This **pulls from GitHub Container Registry**, then:

- Installs Docker (if missing)
- Generates secure secrets
- Starts Postgres + Dockfin
- Opens the dashboard on **port 8000** (80/443 free for Traefik)

Then open:

```text
http://YOUR_SERVER_IP:8000/
```

1. **Register** your admin account  
2. The VPS is added as a server for you  
3. Create a **project** and deploy  

**Update anytime:**

```bash
cd /data/dockfin && sudo docker compose pull && sudo docker compose up -d
```

**Pin a release version:**

```bash
sudo DOCKFIN_VERSION=1.0.9 \
  bash -c 'curl -fsSL https://raw.githubusercontent.com/foisalislambd/dockfin/main/scripts/install.sh | bash'
```

### Useful commands after install

```bash
curl -s http://YOUR_SERVER_IP:8000/health
curl -s http://YOUR_SERVER_IP:8000/api/v1/version

cd /data/dockfin && sudo docker compose logs -f dockfin
cd /data/dockfin && sudo docker compose down
cd /data/dockfin && sudo docker compose up -d
```

Everything important lives in `/data/dockfin`. You don’t need to edit `.env` by hand.

---

## What you can deploy

- Apps — Dockerfile, Compose, image, Railpack, static  
- Databases — Postgres, MySQL, MongoDB, Redis, and more  
- ~360 one-click service templates  
- Git webhooks, env vars, live deploy logs  

---

## Local development

### Docker (build from this repo)

```bash
git clone https://github.com/foisalislambd/dockfin.git
cd dockfin
sudo bash scripts/install-dev.sh
```

Builds `dockfin:local` into `/data/dockfin` (same dir as production; no registry pull).

### Go + Vite (hot reload)

```bash
git clone https://github.com/foisalislambd/dockfin.git
cd dockfin
cp .env.example .env   # set DOCKFIN_MASTER_KEY (32+ chars)

docker compose -f deploy/compose/docker-compose.yml up -d postgres
go run ./cmd/dockfin migrate && go run ./cmd/dockfin serve

cd apps/web && npm install && npm run dev   # http://localhost:5173
```

| Script | Use |
|--------|-----|
| `scripts/install.sh` | **Production** — pull `ghcr.io/foisalislambd/dockfin` |
| `scripts/install-dev.sh` | **Dev** — build local image from source |

---

## Releases

The version lives in [VERSION.md](VERSION.md). A push to `main` publishes a GitHub Release and Docker images only when that `X.Y.Z` is **higher** than the latest `vX.Y.Z` tag. The same version, or a lower one, does not create a release.

Bump the number in `VERSION.md` (for example `1.3.6` → `1.3.7`) in the commit that should be released.

---

## API, metrics, and the CLI

| URL | What |
|-----|------|
| `/openapi.json` | OpenAPI 3.1 generated from the live router |
| `/api/docs` | HTML index of the same routes |
| `/metrics` | Prometheus counters (route pattern, not raw URLs) |
| `/health` | Liveness plus the running version |

Per-IP limits default to 600 requests/minute, 30 auth attempts per 15 minutes, and 120 webhook calls/minute. Set `DOCKFIN_RATE_LIMIT_PER_MINUTE`, `DOCKFIN_AUTH_RATE_LIMIT_PER_15M`, or `DOCKFIN_WEBHOOK_RATE_LIMIT_PER_MINUTE`. Zero disables that bucket. `DOCKFIN_RATE_LIMIT_STORE=postgres` shares that budget across replicas (default `memory`).

`/metrics` is public unless `DOCKFIN_METRICS_TOKEN` is set, in which case Prometheus must send `Authorization: Bearer <token>`.

```bash
dfin --json projects list
dfin apps create --environment <env-uuid> --name web --git https://github.com/org/repo
dfin env set --resource-type application --resource-id <app-uuid> --key PORT --value 8080
dfin logs <deployment-uuid> --follow
```

Audit history is kept for `DOCKFIN_AUDIT_RETENTION_DAYS` (default 90). Owners and admins open it from the account menu.

```bash
bash scripts/e2e-smoke.sh
```

```bash
export DOCKFIN_URL=http://127.0.0.1:8000
export DOCKFIN_TOKEN=...          # API token
dfin overview
dfin freeze <project-uuid> <environment-uuid> --reason "maintenance"
dfin deploy <application-uuid> --override-freeze
dfin unfreeze <project-uuid> <environment-uuid>
dfin logs <deployment-uuid>
```

Commands exit 0 on HTTP 2xx/3xx and 1 otherwise. `DOCKFIN_TEAM` sends `X-Team-ID`.

---

## Docs & help

| Link | |
|------|--|
| [Features](docs/FEATURES.md) | Checklist |
| [Architecture](docs/ARCHITECTURE.md) | Design |
| [Contributing](CONTRIBUTING.md) | PRs |
| [Security](SECURITY.md) | Report issues privately |
| [Changelog](CHANGELOG.md) | Releases |

---

## License

[MIT](LICENSE)
