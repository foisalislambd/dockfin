# Testing

Dockfin’s checks are the Go suite, the dashboard’s node tests, and the production build. CI runs them on every pull request (`.github/workflows/ci.yml`) against Postgres 16 with the race detector.

## Go

```bash
export DOCKFIN_DATABASE_URL=postgres://dockfin:dockfin@localhost:5432/dockfin?sslmode=disable
go test -race -count=1 -coverprofile=coverage.out ./...
go vet ./...
go build ./...
```

Integration tests in `internal/store` and `internal/httpapi` open a throwaway database (`internal/testkit`) and skip when `DOCKFIN_DATABASE_URL` is unset or Postgres is down. They cover:

- Exclusive job claims (`FOR UPDATE SKIP LOCKED`), an empty queue, and crash reclaim before and after the last attempt
- Deletion tombstones, rollback when the job payload is not JSON, a second delete while a job is queued, and the dashboard error after a worker dies
- Power actions that die on the last attempt settle to `error`
- Authn/authz: no session, registration locked after the first account, cookie flags, CORS does not reflect an unknown origin, CSP and `nosniff`, a project name that looks like SQL, unsigned webhooks, a view-only member versus another project, env vars on that project, `X-Team-ID` for a team the user is not on, and an oversized body
- Shell quoting round-trips, including quotes and command substitutions
- Domain normalization rejects spaces, newlines, `..`, and `javascript:` URLs

Fuzz targets (seeds run on every `go test`; a short `-fuzz` run is local):

```bash
go test ./internal/proxy/ -run '^$' -fuzz=FuzzNormalizeDomains -fuzztime=10s
go test ./internal/sshx/ -run '^$' -fuzz=FuzzShellQuote -fuzztime=10s
go test ./internal/services/ -run '^$' -fuzz=FuzzParseComposeUnits -fuzztime=10s
```

CI fails if total statement coverage drops below 20%. The race run on this branch is 21.4% of statements. That floor is the current suite, not a claim that every handler is covered. New packages should add tests rather than lower the number.

`go test ./internal/httpapi/ -run TestOpenAPIFileMatchesRouter` checks the committed OpenAPI file. `info.version` stays `dev`.

## Dashboard

```bash
cd apps/web
npm test   # node:test for search, catalog, and route skeletons
npm run build
```

## What this round ran

`go test` on the store, HTTP, proxy, ssh, and services packages, eight-second fuzz runs of the three parsers, `npm test` (23 tests), and `npm run build`. The race run is the CI command above.
