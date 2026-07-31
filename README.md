# Goolify

**Deploy your apps on your own server** — like Heroku or Vercel, but self-hosted.

Goolify is an open-source PaaS. You install it on a VPS, open the dashboard in your browser, connect servers over SSH, and deploy apps, databases, and one-click services with Docker.

- **One Docker image** — API + web dashboard together  
- **MIT licensed** — no paywall, no lock-in  
- **Your data stays on your machines** under `/data/goolify/…`

| Pull from | Image |
|-----------|--------|
| [Docker Hub](https://hub.docker.com/r/foisalislambd/goolify) | `foisalislambd/goolify:latest` |
| GitHub Packages | `ghcr.io/foisalislambd/goolify:latest` |

---

## What you can do

- Deploy apps (Dockerfile, Compose, Docker image, Nixpacks, static sites)
- Run managed databases (Postgres, MySQL, MongoDB, Redis, and more)
- Launch ~360 one-click services from a template catalog
- Use teams, projects, environments, env vars, and Git webhooks
- Stream deploy logs live in the UI

---

## Production server (recommended)

Use a fresh **Ubuntu/Debian VPS** with a public IP. Root (or sudo) access is enough.

### Option A — Installer (easiest)

```bash
curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash
```

What it does for you:

1. Installs Docker if needed  
2. Creates `/data/goolify` with a secure `.env`  
3. Starts **Postgres + Goolify** with Compose  
4. Publishes the dashboard on **port 80** (API on the same process)

When it finishes, open:

```text
http://YOUR_SERVER_IP/
```

1. **Register** the first admin account  
2. The VPS is usually added as a server automatically (or use **Servers → bootstrap**)  
3. Create a **Project** → deploy an app or a one-click service  

Data lives in `/data/goolify`. To stop or update later:

```bash
cd /data/goolify
docker compose pull
docker compose up -d
```

### Option B — Docker Compose yourself

Create a folder (e.g. `/data/goolify`) with `.env` and `docker-compose.yml`:

**`.env`** (change secrets):

```env
GOOLIFY_ENV=production
GOOLIFY_HTTP_ADDR=:8000
GOOLIFY_DATABASE_URL=postgres://goolify:CHANGE_DB_PASSWORD@postgres:5432/goolify?sslmode=disable
GOOLIFY_MASTER_KEY=CHANGE_ME_TO_AT_LEAST_32_CHARACTERS_LONG!!
GOOLIFY_CORS_ORIGINS=*
GOOLIFY_PUBLIC_URL=http://YOUR_SERVER_IP
GOOLIFY_PUBLIC_IP=YOUR_SERVER_IP
GOOLIFY_BOOTSTRAP_SELF=1
GOOLIFY_DATA_DIR=/data
GOOLIFY_TEMPLATES_DIR=/app/templates
GOOLIFY_WEB_DIR=/app/web
POSTGRES_PASSWORD=CHANGE_DB_PASSWORD
```

**`docker-compose.yml`:**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: goolify
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: goolify
    volumes:
      - goolify-pg:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U goolify"]
      interval: 5s
      timeout: 5s
      retries: 20
    restart: unless-stopped

  goolify:
    image: foisalislambd/goolify:latest
    env_file: .env
    environment:
      GOOLIFY_DATABASE_URL: ${GOOLIFY_DATABASE_URL}
      GOOLIFY_HTTP_ADDR: ":8000"
    ports:
      - "80:8000"
      - "8000:8000"
    volumes:
      - goolify-data:/data
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

volumes:
  goolify-pg:
  goolify-data:
```

Then:

```bash
docker compose up -d
```

Open `http://YOUR_SERVER_IP/` and register.

### Useful checks

```bash
curl -s http://YOUR_SERVER_IP/health
curl -s http://YOUR_SERVER_IP/api/v1/version
docker compose -f /data/goolify/docker-compose.yml logs -f goolify
```

### Updating to a new release

```bash
cd /data/goolify
docker compose pull
docker compose up -d
```

Pin a version if you prefer: `foisalislambd/goolify:1.0.1` instead of `:latest`.

---

## Local development

### Prerequisites

- Go 1.26+
- Node.js 22+
- Docker (for Postgres)

### Setup

```bash
git clone https://github.com/foisalislambd/goolify.git
cd goolify
cp .env.example .env
# Set GOOLIFY_MASTER_KEY to 32+ characters
```

Start Postgres (example):

```bash
docker compose -f deploy/compose/docker-compose.yml up -d postgres
```

API (serves the UI if you point `GOOLIFY_WEB_DIR` at a built SPA):

```bash
go run ./cmd/goolify migrate
go run ./cmd/goolify serve
```

Dashboard in dev mode:

```bash
cd apps/web
npm install
npm run build   # optional: serve from API via GOOLIFY_WEB_DIR=apps/web/dist
npm run dev     # http://localhost:5173 with API proxy
```

Health: http://localhost:8000/health

---

## Releases & Docker

Every push to `main` can cut a release automatically:

- Git tag `vX.Y.Z` + [GitHub Release](https://github.com/foisalislambd/goolify/releases)
- Docker image with the **same** version on Docker Hub and GHCR  

Version scheme: `1.0.0` → … → `1.0.9` → `1.1.0` (patch/minor digits 0–9).

### Skip a release

If you push to `main` but **do not** want a new version / Docker publish, put one of these in the **commit message**:

```text
[skip release]
[skip-release]
[skip_release]
```

Example:

```bash
git commit -m "docs: fix typo [skip release]"
git push origin main
```

The Release workflow will not run for that push. (CI tests can still run.)

To release manually: GitHub → **Actions** → **Release** → **Run workflow**.

---

## Architecture (simple)

```text
Browser  →  Goolify (API + dashboard)
                 │
                 │  SSH
                 ▼
         Your Docker hosts (Traefik, apps, DBs)
```

Configs for workloads live on the target servers under `/data/goolify/…`. If you stop Goolify later, containers can still be managed with Docker/Compose.

---

## CLI (`glfy`)

```bash
export GOOLIFY_URL=http://YOUR_SERVER_IP
export GOOLIFY_TOKEN=<token-from-login>

glfy version
glfy health
glfy servers
glfy apps
glfy deploy <application-uuid>
glfy logs <deployment-uuid>
```

---

## Documentation

| Doc | What it’s for |
|-----|----------------|
| [docs/README.md](docs/README.md) | Deeper install & first deploy |
| [docs/FEATURES.md](docs/FEATURES.md) | Feature checklist |
| [docs/VPS-SMOKE-TEST.md](docs/VPS-SMOKE-TEST.md) | Automated VPS smoke test |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Design overview |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [SECURITY.md](SECURITY.md) | Report vulnerabilities |
| [CHANGELOG.md](CHANGELOG.md) | What changed per release |

---

## Contributing

```bash
go test ./...
cd apps/web && npm ci && npm run build
```

Please read [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).

---

## Security

Please do **not** open public issues for sensitive bugs. See [SECURITY.md](SECURITY.md).

---

## License

[MIT](LICENSE)

Goolify is an independent open-source project inspired by the same problem space as Coolify — not affiliated with Coolify’s trademark or codebase.
