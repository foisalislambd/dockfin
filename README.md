# Goolify

**Deploy apps on your own server** — like Heroku / Vercel, but self-hosted.

One Docker image = API + dashboard. MIT license. Your data stays on your VPS.

| Registry | Image |
|----------|--------|
| [Docker Hub](https://hub.docker.com/r/foisalislambd/goolify) | `foisalislambd/goolify:latest` |
| GHCR | `ghcr.io/foisalislambd/goolify:latest` |

---

## Install on a production server

You need a **Ubuntu/Debian VPS** with a public IP. That’s it.

### One command (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash
```

This automatically:

- Installs Docker (if missing)
- Generates secure secrets
- Starts Postgres + Goolify
- Opens the dashboard on **port 80**

Then open:

```text
http://YOUR_SERVER_IP/
```

1. **Register** your admin account  
2. The VPS is added as a server for you  
3. Create a **project** and deploy  

**Update anytime** (still no manual config):

```bash
cd /data/goolify && sudo docker compose pull && sudo docker compose up -d
```

---

## Docker pull & run

Already have Docker? Same installer still does the full stack for you (Goolify needs Postgres):

```bash
docker pull foisalislambd/goolify:latest
curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash
```

Or pin a version:

```bash
sudo GOOLIFY_IMAGE=foisalislambd/goolify:1.0.1 \
  bash -c 'curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | bash'
```

### Useful commands after install

```bash
# Status
curl -s http://YOUR_SERVER_IP/health
curl -s http://YOUR_SERVER_IP/api/v1/version

# Logs
cd /data/goolify && sudo docker compose logs -f goolify

# Stop / start
cd /data/goolify && sudo docker compose down
cd /data/goolify && sudo docker compose up -d
```

Everything important lives in `/data/goolify` (created for you). You don’t need to edit `.env` by hand.

---

## What you can deploy

- Apps — Dockerfile, Compose, image, Nixpacks, static  
- Databases — Postgres, MySQL, MongoDB, Redis, and more  
- ~360 one-click service templates  
- Git webhooks, env vars, live deploy logs  

---

## Local development (contributors)

```bash
git clone https://github.com/foisalislambd/goolify.git
cd goolify
cp .env.example .env   # set GOOLIFY_MASTER_KEY (32+ chars)

docker compose -f deploy/compose/docker-compose.yml up -d postgres
go run ./cmd/goolify migrate && go run ./cmd/goolify serve

cd apps/web && npm install && npm run dev   # http://localhost:5173
```

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
