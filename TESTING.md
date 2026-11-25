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

Fuzz targets (seeds run on every `go test`; longer `-fuzz` runs are local):

```bash
go test ./internal/proxy/ -run '^$' -fuzz=FuzzNormalizeDomains -fuzztime=30s
go test ./internal/sshx/ -run '^$' -fuzz=FuzzShellQuote -fuzztime=30s
go test ./internal/services/ -run '^$' -fuzz=FuzzParseComposeUnits -fuzztime=30s
go test ./internal/services/ -run '^$' -fuzz=FuzzParseEnvFile -fuzztime=120s
go test ./internal/githubrunner/ -run '^$' -fuzz=FuzzStartArgs -fuzztime=120s
go test ./internal/git/ -run '^$' -fuzz=FuzzVerifyGitHubSignature -fuzztime=90s
go test ./internal/git/ -run '^$' -fuzz=FuzzParseWebhook -fuzztime=120s
go test ./internal/netsafe/ -run '^$' -fuzz=FuzzValidateHTTPURL -fuzztime=120s
go test ./internal/git/ -run '^$' -fuzz=FuzzValidateRemote -fuzztime=120s
```

`TestEveryRouteAuthzMatrix` walks `chi` and compares the route list to `internal/httpapi/testdata/authz_routes.txt`. A route with no line fails the build. Regenerate with `DOCKFIN_UPDATE_AUTHZ=1` only after reviewing the class (public, auth, session, admin).

`go test -coverpkg=./...` records statements hit from another package (the store tests live in `internal/storetests` so they do not import-cycle with `internal/testkit`). CI dedupes the profile, because each package run appends a copy, and fails below 50%.

```bash
staticcheck ./...          # config: staticcheck.conf
gosec -quiet -conf .gosec.json ./...
govulncheck ./...
```

`go test ./internal/httpapi/ -run TestOpenAPIFileMatchesRouter` checks the committed OpenAPI file. `info.version` stays `dev`.

## Dashboard

```bash
cd apps/web
npm test   # node:test plus Vitest and Testing Library
npm run build
npx playwright test   # needs the API on :8000 and Chromium
```

Vitest covers the login email error, form blur, the info hint, modal focus and Escape, API error parsing (including that a login 401 does not clear the session), skeletons, and deletion Retry / Forget anyway. Playwright signs in on a fresh database, runs axe (serious and critical, color-contrast excluded because muted gray text is documented) on the core pages, checks the project dialog, the server form, the team invite field, and that the page does not overflow horizontally.

## What this round ran

Statement coverage, using the same dedupe as CI (`go test -race -count=1 -coverpkg=./...`): 21.4% at the start of the round, then 30.4%, 37%, 42.4%, 48.6%, 50.1% (14411/28754), and **52.9% (15233/28770)** on the race run that matches CI. The CI floor stays 50%. Go `Test` functions went from 339 to 417. `npm test` is 23 node:test cases and 33 Vitest cases (was 10). Playwright stays at 2.

Fuzz, each for at least two minutes except the GitHub signature run at 90 seconds: `FuzzParseEnvFile` passed (about 8.8 million executions). `FuzzStartArgs`, `FuzzParseWebhook`, and `FuzzValidateHTTPURL` passed at 120 seconds. `FuzzVerifyGitHubSignature` passed at 90 seconds. `FuzzValidateRemote` failed on `file:/0`, `169.254.169.254/0`, `http://0169.254.169.254`, and `http://169.254.169.2540`. Those seeds stay in the corpus. After the fix, a 120 second run passed (about 14 million executions).

`npm test`: 23 node:test cases and 33 Vitest cases. `npm audit --audit-level=high` reports 0 vulnerabilities. Playwright: 2 passed (desktop 1440×900 and Pixel 5) covering login, the signed-in pages with axe, a project, deploy, domains, env vars, logs, backups, delete with forget, the server form, team invite, settings, and webhook URL validation.

`staticcheck`, `gosec -conf .gosec.json`, and `govulncheck` exit 0. The gosec config must put the rule list in `global.exclude` as a comma-separated string; a top-level array is ignored and the scan reports the excluded rules.
