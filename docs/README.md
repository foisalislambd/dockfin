# Goolify docs

## Install (production)

Pulls **`ghcr.io/foisalislambd/goolify:latest`** from GitHub Packages, then starts the stack:

```bash
curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash
```

Then open `http://YOUR_SERVER_IP:8000/` and register.

Pin a version: `sudo GOOLIFY_VERSION=1.0.9 bash -c 'curl -fsSL …/install.sh | bash'`

Full user guide: [README](../README.md)

## Guides

| Doc | Description |
|-----|-------------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Control plane design |
| [FEATURES.md](FEATURES.md) | Feature parity checklist |
| [VPS-SMOKE-TEST.md](VPS-SMOKE-TEST.md) | One-click VPS smoke test |

## First steps after install

1. Open the UI → **Register** admin  
2. Create a **Project**  
3. Deploy an app or a one-click service  

(First register usually auto-adds this VPS as a server.)

## Development

Docker from this repo (no GHCR pull):

```bash
sudo bash scripts/install-dev.sh
```

Or hot-reload:

```bash
cp .env.example .env
docker compose -f deploy/compose/docker-compose.yml up -d postgres
go run ./cmd/goolify migrate
go run ./cmd/goolify serve
cd apps/web && npm install && npm run dev
```

| Script | Environment |
|--------|-------------|
| `scripts/install.sh` | Production (GHCR) |
| `scripts/install-dev.sh` | Local build |

## Webhooks

```
POST /api/v1/webhooks/git/{application_uuid}?provider=github
```

Set a webhook secret via `POST /api/v1/applications/{id}/webhook-secret`.

## Shared environment variables

- `{{team.KEY}}` · `{{project.KEY}}` · `{{environment.KEY}}` · `{{server.KEY}}`

## CLI

```bash
export GOOLIFY_URL=http://YOUR_SERVER_IP
export GOOLIFY_TOKEN=<token-from-login>
glfy apps
glfy deploy <app-uuid>
```
