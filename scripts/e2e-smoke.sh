#!/usr/bin/env bash
# Boots the dev Postgres, migrates, and exercises health, CSP, metrics auth,
# registration, and the audit log. Requires Docker and Go.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! docker info >/dev/null 2>&1; then
  echo "Docker is not available. Start Docker and rerun scripts/e2e-smoke.sh." >&2
  exit 1
fi

compose=(docker compose -f deploy/compose/docker-compose.dev.yml)
"${compose[@]}" up -d postgres
for _ in $(seq 1 30); do
  if "${compose[@]}" exec -T postgres pg_isready -U dockfin >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
"${compose[@]}" exec -T postgres pg_isready -U dockfin

export DOCKFIN_ENV=development
export DOCKFIN_DATABASE_URL="${DOCKFIN_DATABASE_URL:-postgres://dockfin:dockfin@127.0.0.1:5432/dockfin?sslmode=disable}"
export DOCKFIN_MASTER_KEY="${DOCKFIN_MASTER_KEY:-e2e-smoke-master-key-must-be-32b!!}"
export DOCKFIN_HTTP_ADDR="${DOCKFIN_HTTP_ADDR:-127.0.0.1:18000}"
export DOCKFIN_PUBLIC_URL="http://${DOCKFIN_HTTP_ADDR}"
export DOCKFIN_COOKIE_SECURE=0
export DOCKFIN_BOOTSTRAP_SELF=0
export DOCKFIN_METRICS_TOKEN="${DOCKFIN_METRICS_TOKEN:-e2e-metrics-token}"
export DOCKFIN_RATE_LIMIT_STORE=postgres
export DOCKFIN_WEB_DIR="${DOCKFIN_WEB_DIR:-$PWD/apps/web/dist}"

go run ./cmd/dockfin migrate

log="$(mktemp)"
go run ./cmd/dockfin serve >"$log" 2>&1 &
pid=$!
cleanup() {
  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" >/dev/null 2>&1 || true
  rm -f "$log"
}
trap cleanup EXIT

base="http://${DOCKFIN_HTTP_ADDR}"
for _ in $(seq 1 40); do
  if curl -sf "$base/health" >/dev/null; then
    break
  fi
  sleep 0.5
done

echo "-- health"
curl -sf "$base/health" | grep -q '"status":"ok"'

echo "-- csp"
csp="$(curl -sI "$base/health" | tr -d '\r' | awk 'BEGIN{IGNORECASE=1} /^content-security-policy:/ {print}')"
echo "$csp" | grep -q "script-src 'self'"
echo "$csp" | grep -q "worker-src 'self' blob:"

echo "-- metrics token"
code="$(curl -s -o /dev/null -w '%{http_code}' "$base/metrics")"
test "$code" = "401"
curl -sf -H "Authorization: Bearer ${DOCKFIN_METRICS_TOKEN}" "$base/metrics" | grep -q dockfin_http_requests_total

echo "-- register and audit"
email="e2e-$(date +%s)@example.com"
curl -sf -c /tmp/dockfin-e2e.cookies -H 'Content-Type: application/json' \
  -d "{\"email\":\"$email\",\"name\":\"E2E\",\"password\":\"e2e-password\"}" \
  "$base/api/v1/auth/register" >/dev/null
curl -sf -b /tmp/dockfin-e2e.cookies "$base/api/v1/audit-logs?category=auth" | grep -q register

echo "e2e smoke passed"
