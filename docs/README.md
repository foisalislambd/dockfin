# Goolify docs

## Guides

| Doc | Description |
|-----|-------------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Control plane design |
| [FEATURES.md](FEATURES.md) | Feature parity checklist |
| [VPS-SMOKE-TEST.md](VPS-SMOKE-TEST.md) | One-click VPS install + smoke test |

## Install (production)

```bash
sudo bash scripts/install.sh
```

Or from a source checkout after configuring images / building locally — see the [README](../README.md).

## Development

```bash
cp .env.example .env
docker compose -f deploy/compose/docker-compose.dev.yml up -d
go run ./cmd/goolify migrate
go run ./cmd/goolify serve
cd apps/web && npm install && npm run dev
```

## First deploy happy path

1. Open the UI → **Servers** (add this host / SSH key) → **Projects**
2. Add an SSH private key
3. Add a server → **Validate** → **Start Traefik proxy**
4. Create a project (production environment is created automatically)
5. Create an application (`dockerimage` `nginx:alpine` or a git Dockerfile app)
6. Deploy → watch SSE logs on the application detail page

## Webhooks

```
POST /api/v1/webhooks/git/{application_uuid}?provider=github
```

Set a webhook secret via `POST /api/v1/applications/{id}/webhook-secret`.

- GitHub: `X-Hub-Signature-256`
- GitLab: `X-Gitlab-Token`

## Shared environment variables

Reference secrets as:

- `{{team.KEY}}`
- `{{project.KEY}}`
- `{{environment.KEY}}`
- `{{server.KEY}}`

## CLI

```bash
export GOOLIFY_URL=http://localhost:8000
export GOOLIFY_TOKEN=<session-token-from-login>
glfy apps
glfy deploy <app-uuid>
glfy logs <deployment-uuid>
```

## Templates

The service catalog loads from `GOOLIFY_TEMPLATES_DIR` or `templates/compose`, plus built-in stubs.
