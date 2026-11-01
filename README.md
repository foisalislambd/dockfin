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

## Dashboard

The console is a Vite + React app. The API serves the built files from `DOCKFIN_WEB_DIR` (the production image bakes `apps/web/dist` in the same way). `cd apps/web && npm run dev` is the hot-reload UI on port 5173; `npm run build` is what `scripts/install-dev.sh` and the Docker image use.

The shell is a full-width top bar (team, search, theme, account) with a sectioned sidebar underneath. `⌘K` / `Ctrl+K` opens search across pages and resources. `g` then a letter jumps to a page (`g` `p` projects, `g` `h` servers). `?` lists shortcuts. The first signed-in visit walks through adding a server (including this machine), creating a project, and deploying an app, database, or one-click service.

Layout ideas — top bar, grouped navigation, command palette, go-to keys — are a clean-room reimplementation inspired by the [Antirion](https://github.com/eliasbiondo/antirion) console (Apache-2.0). No Antirion source is included. Dockfin keeps its own blue palette, logo, and fonts. See [apps/web/NOTICE](apps/web/NOTICE).

<p>
  <img src="docs/screenshots/login.png" alt="Sign in" width="720" />
</p>
<p>
  <img src="docs/screenshots/dashboard.png" alt="Overview with first-run steps" width="720" />
</p>
<p>
  <img src="docs/screenshots/command-palette.png" alt="Command palette" width="720" />
</p>
<p>
  <img src="docs/screenshots/projects.png" alt="Projects" width="720" />
</p>
<p>
  <img src="docs/screenshots/servers.png" alt="Servers empty state" width="720" />
</p>
<p>
  <img src="docs/screenshots/settings-dark.png" alt="Settings in dark mode" width="720" />
</p>

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
