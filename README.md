# Goolify

**Deploy apps on your own server** — like Heroku / Vercel, but self-hosted.

One Docker image = API + dashboard. MIT license. Your data stays on your VPS.

| Registry | Image |
|----------|--------|
| **GitHub Packages (GHCR)** — production default | `ghcr.io/foisalislambd/goolify:latest` |
| [Docker Hub](https://hub.docker.com/r/foisalislambd/goolify) | `foisalislambd/goolify:latest` |

---

## Install on a production server

You need a **Ubuntu/Debian VPS** with a public IP. That’s it.

### One command (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash
```

This **pulls from GitHub Container Registry**, then:

- Installs Docker (if missing)
- Generates secure secrets
- Starts Postgres + Goolify
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
cd /data/goolify && sudo docker compose pull && sudo docker compose up -d
```

**Pin a release version:**

```bash
sudo GOOLIFY_VERSION=1.0.9 \
  bash -c 'curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | bash'
```

### Useful commands after install

```bash
curl -s http://YOUR_SERVER_IP:8000/health
curl -s http://YOUR_SERVER_IP:8000/api/v1/version

cd /data/goolify && sudo docker compose logs -f goolify
cd /data/goolify && sudo docker compose down
cd /data/goolify && sudo docker compose up -d
```

Everything important lives in `/data/goolify`. You don’t need to edit `.env` by hand.

---

## What you can deploy

- Apps — Dockerfile, Compose, image, Nixpacks, static  
- Databases — Postgres, MySQL, MongoDB, Redis, and more  
- ~360 one-click service templates  
- Git webhooks, env vars, live deploy logs  

---

## Local development

### Docker (build from this repo)

```bash
git clone https://github.com/foisalislambd/goolify.git
cd goolify
sudo bash scripts/install-dev.sh
```

Builds `goolify:local` into `/data/goolify` (same dir as production; no registry pull).

### Go + Vite (hot reload)

```bash
git clone https://github.com/foisalislambd/goolify.git
cd goolify
cp .env.example .env   # set GOOLIFY_MASTER_KEY (32+ chars)

docker compose -f deploy/compose/docker-compose.yml up -d postgres
go run ./cmd/goolify migrate && go run ./cmd/goolify serve

cd apps/web && npm install && npm run dev   # http://localhost:5173
```

| Script | Use |
|--------|-----|
| `scripts/install.sh` | **Production** — pull `ghcr.io/foisalislambd/goolify` |
| `scripts/install-dev.sh` | **Dev** — build local image from source |

---

## Releases

Push to `main` publishes the same version to GitHub Release + Docker Hub + GHCR  
(`1.0.0` … `1.0.9` → `1.1.0`).

**Skip a release** — put this in the commit message:

```text
[skip release]
```

Example: `git commit -m "docs: tweak README [skip release]"`

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
