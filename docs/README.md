# Goolify docs

## Install (production)

```bash
curl -fsSL https://raw.githubusercontent.com/goolify/goolify/main/scripts/install.sh | sudo bash
```

Or locally from this repo after building images:

```bash
sudo GOOLIFY_DIR=/data/goolify bash scripts/install.sh
```

## Development

```bash
cp .env.example .env
docker compose -f deploy/compose/docker-compose.dev.yml up -d
go run ./cmd/goolify migrate
go run ./cmd/goolify serve
cd apps/web && npm run dev
```

## First deploy happy path

1. Open UI → Onboarding (or Dashboard CTA)
2. Add SSH private key
3. Add server → Validate → Start Traefik proxy
4. Create project (production env saved automatically)
5. Create application (dockerimage `nginx:alpine` or git Dockerfile)
6. Deploy → watch SSE logs on application detail page

## Webhooks

```
POST /api/v1/webhooks/git/{application_uuid}?provider=github
```

Set a webhook secret via `POST /api/v1/applications/{id}/webhook-secret`.  
GitHub: `X-Hub-Signature-256`. GitLab: `X-Gitlab-Token`.

## Shared env vars

Reference as `{{team.KEY}}`, `{{project.KEY}}`, `{{environment.KEY}}`, `{{server.KEY}}`.

## CLI

```bash
export GOOLIFY_URL=http://localhost:8080
export GOOLIFY_TOKEN=<session-token-from-login>
glfy apps
glfy deploy <app-uuid>
glfy logs <deployment-uuid>
```

## Templates

Service catalog loads from `GOOLIFY_TEMPLATES_DIR` or `coolify/templates/compose` when present, plus built-in stubs.
