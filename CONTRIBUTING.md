# Contributing to Dockfin

Thanks for contributing. This document explains how to develop, test, and submit changes.

## Code of Conduct

Participation is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Development setup

### Requirements

- Go 1.26+
- Node.js 22+
- Docker (recommended for Postgres/Redis)

### Run locally

```bash
cp .env.example .env
docker compose -f deploy/compose/docker-compose.dev.yml up -d
go run ./cmd/dockfin migrate
go run ./cmd/dockfin serve
```

In another terminal:

```bash
cd apps/web
npm install
npm run dev
```

## Project conventions

- **Language:** English for code comments, docs, commit messages, and PRs
- **Backend:** Go modules under `internal/`; HTTP in `internal/httpapi`
- **Frontend:** React + TypeScript under `apps/web`
- **Migrations:** Goose SQL only in `migrations/` (embedded into the binary via `migrations/embed.go`)
- **Secrets:** never commit `.env`, private keys, or real tokens
- **SSH:** prefer `sshx.RunArgs` (argv) over shell-interpolated strings

## Before you open a PR

1. Format / build:

```bash
go test ./...
go build ./...
cd apps/web && npm run build
```

2. Update docs if you change user-facing behavior (`README.md`, `docs/FEATURES.md`, etc.).
3. Keep PRs focused — one feature or fix per PR when possible.

## Commit messages

Use short, imperative subjects:

- `Add webhook HMAC verification`
- `Fix force_rebuild docker build args`
- `Document VPS smoke test script`

## Pull request checklist

- [ ] Tests / build pass locally
- [ ] No secrets in the diff
- [ ] Docs / FEATURES checklist updated if needed
- [ ] Describes *why* the change exists

## Reporting bugs

Use GitHub Issues with:

- Dockfin version / commit
- OS and Docker version
- Steps to reproduce
- Relevant logs (redact secrets)

Security issues: see [SECURITY.md](SECURITY.md).

## Feature requests

Open an issue with the use case, Coolify parity reference (if any), and whether you can help implement it.

## License

By contributing, you agree that your contributions are licensed under the MIT License.
