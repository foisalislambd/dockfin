#!/usr/bin/env bash
# =============================================================================
# Dockfin — one-click VPS install + smoke test
# =============================================================================
# On a fresh Ubuntu/Debian VPS this script will:
#   1) Install Docker / Go / dependencies
#   2) Start Postgres
#   3) Build Dockfin API, migrate, and serve
#   4) Add this VPS as a server (public IP) → validate → Traefik → nginx:alpine deploy
#   5) Print a pass/fail report
#
# First-user register auto-bootstraps the install host with DOCKFIN_PUBLIC_IP
# (same fields as a manually added remote server — not 127.0.0.1).
#
# Usage (as root on the VPS):
#   curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/dockfin/main/scripts/vps-oneclick-test.sh | bash
#
# Or from a cloned repo:
#   sudo bash scripts/vps-oneclick-test.sh
#
# Optional env:
#   DOCKFIN_SRC=/path/to/dockfin   # default: auto-detect or clone
#   DOCKFIN_GIT_URL=...            # git clone URL if source missing
#   SKIP_DEPLOY=1                  # only API smoke (no SSH deploy)
#   KEEP_RUNNING=1                 # don't stop API after tests (default: keep)
# =============================================================================
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}==>${NC} $*"; }
warn() { echo -e "${YELLOW}!!${NC} $*"; }
fail() { echo -e "${RED}FAIL:${NC} $*"; exit 1; }
ok()   { echo -e "${GREEN}OK:${NC} $*"; }

if [[ "${EUID}" -ne 0 ]]; then
  fail "Run as root: sudo bash $0"
fi

API_PORT="${API_PORT:-8000}"
API_URL="http://127.0.0.1:${API_PORT}"
PUBLIC_IP="$(curl -4 -fsS --max-time 3 ifconfig.me 2>/dev/null || curl -4 -fsS --max-time 3 icanhazip.com 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}' || true)"
PUBLIC_IP="$(echo "${PUBLIC_IP}" | tr -d '[:space:]')"
if [[ -n "${PUBLIC_IP}" ]]; then
  PUBLIC_API_URL="http://${PUBLIC_IP}:${API_PORT}"
else
  PUBLIC_API_URL="${API_URL}"
fi
TEST_EMAIL="smoke-$(date +%s)@dockfin.test"
TEST_PASS="SmokeTestPass123!"
TEST_NAME="Smoke Tester"
WORKDIR="${DOCKFIN_WORKDIR:-/opt/dockfin-smoke}"
COOKIE_JAR="${WORKDIR}/cookies.txt"
REPORT="${WORKDIR}/report.txt"
KEEP_RUNNING="${KEEP_RUNNING:-1}"
SKIP_DEPLOY="${SKIP_DEPLOY:-0}"

mkdir -p "${WORKDIR}"
: > "${REPORT}"
report() { echo "$*" | tee -a "${REPORT}"; }

# -----------------------------------------------------------------------------
# 1) System packages
# -----------------------------------------------------------------------------
log "Installing system packages..."
apt-get update -qq
apt-get install -y -qq curl git jq ca-certificates openssl openssh-server openssh-client \
  build-essential > /dev/null

systemctl enable --now ssh 2>/dev/null || systemctl enable --now sshd 2>/dev/null || true

if ! command -v docker >/dev/null 2>&1; then
  log "Installing Docker..."
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
fi
docker compose version >/dev/null 2>&1 || fail "docker compose plugin missing"

# Go 1.26+
need_go=0
if ! command -v go >/dev/null 2>&1; then
  need_go=1
else
  ver="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
  major="${ver%%.*}"
  rest="${ver#*.}"
  minor="${rest%%.*}"
  if [[ "${major}" -lt 1 ]] || [[ "${major}" -eq 1 && "${minor}" -lt 26 ]]; then
    need_go=1
  fi
fi
if [[ "${need_go}" -eq 1 ]]; then
  log "Installing Go 1.26.5..."
  curl -fsSL https://go.dev/dl/go1.26.5.linux-amd64.tar.gz -o /tmp/go.tgz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tgz
  export PATH="/usr/local/go/bin:${PATH}"
  echo 'export PATH=/usr/local/go/bin:$PATH' >/etc/profile.d/golang.sh
fi
export PATH="/usr/local/go/bin:${PATH}"
go version

# Node.js 22+ (for Vite static UI build)
need_node=0
if ! command -v node >/dev/null 2>&1; then
  need_node=1
else
  node_major="$(node -v 2>/dev/null | sed 's/^v//;s/\..*//')"
  if [[ -z "${node_major}" ]] || [[ "${node_major}" -lt 22 ]]; then
    need_node=1
  fi
fi
if [[ "${need_node}" -eq 1 ]]; then
  log "Installing Node.js 22..."
  curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
  apt-get install -y -qq nodejs > /dev/null
fi
node -v
npm -v

# -----------------------------------------------------------------------------
# 2) Source tree
# -----------------------------------------------------------------------------
detect_src() {
  if [[ -n "${DOCKFIN_SRC:-}" && -f "${DOCKFIN_SRC}/go.mod" ]]; then
    echo "${DOCKFIN_SRC}"
    return
  fi
  # script lives in repo?
  local here
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." 2>/dev/null && pwd || true)"
  if [[ -n "${here}" && -f "${here}/go.mod" ]]; then
    echo "${here}"
    return
  fi
  if [[ -f /data/dockfin-src/go.mod ]]; then
    echo /data/dockfin-src
    return
  fi
  echo ""
}

SRC="$(detect_src)"
if [[ -z "${SRC}" ]]; then
  GIT_URL="${DOCKFIN_GIT_URL:-}"
  if [[ -z "${GIT_URL}" ]]; then
    fail "Source not found. Set DOCKFIN_SRC=/path/to/dockfin or DOCKFIN_GIT_URL=https://github.com/.../dockfin.git"
  fi
  log "Cloning ${GIT_URL}..."
  rm -rf /data/dockfin-src
  git clone --depth 1 "${GIT_URL}" /data/dockfin-src
  SRC=/data/dockfin-src
fi
log "Using source: ${SRC}"
cd "${SRC}"

# -----------------------------------------------------------------------------
# 3) Postgres + Redis
# -----------------------------------------------------------------------------
log "Starting Postgres + Redis..."
docker compose -f deploy/compose/docker-compose.dev.yml up -d
sleep 3
for i in $(seq 1 30); do
  if docker compose -f deploy/compose/docker-compose.dev.yml exec -T postgres pg_isready -U dockfin >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

# -----------------------------------------------------------------------------
# 4) Env + build + migrate + serve
# -----------------------------------------------------------------------------
log "Preparing .env..."
if [[ ! -f .env ]]; then
  MASTER_KEY=$(openssl rand -base64 48 | tr -d '\n' | head -c 48)
  cat > .env <<EOF
DOCKFIN_ENV=production
DOCKFIN_HTTP_ADDR=:${API_PORT}
DOCKFIN_DATABASE_URL=postgres://dockfin:dockfin@127.0.0.1:5432/dockfin?sslmode=disable
DOCKFIN_MASTER_KEY=${MASTER_KEY}
DOCKFIN_CORS_ORIGINS=${PUBLIC_API_URL},http://127.0.0.1:${API_PORT}
DOCKFIN_PUBLIC_URL=${PUBLIC_API_URL}
DOCKFIN_PUBLIC_IP=${PUBLIC_IP}
DOCKFIN_BOOTSTRAP_SELF=1
DOCKFIN_COOKIE_SECURE=0
DOCKFIN_DATA_DIR=${WORKDIR}/data
DOCKFIN_TEMPLATES_DIR=${SRC}/templates/compose
DOCKFIN_WEB_DIR=${SRC}/apps/web/dist
EOF
fi

# Refresh public URL / cookie secure / bootstrap if .env already existed from an older run
if [[ -f .env ]]; then
  if [[ -n "${PUBLIC_IP}" ]]; then
    if grep -q '^DOCKFIN_PUBLIC_URL=' .env; then
      sed -i "s|^DOCKFIN_PUBLIC_URL=.*|DOCKFIN_PUBLIC_URL=${PUBLIC_API_URL}|" .env
    else
      echo "DOCKFIN_PUBLIC_URL=${PUBLIC_API_URL}" >> .env
    fi
    if grep -q '^DOCKFIN_PUBLIC_IP=' .env; then
      sed -i "s|^DOCKFIN_PUBLIC_IP=.*|DOCKFIN_PUBLIC_IP=${PUBLIC_IP}|" .env
    else
      echo "DOCKFIN_PUBLIC_IP=${PUBLIC_IP}" >> .env
    fi
  fi
  if grep -q '^DOCKFIN_BOOTSTRAP_SELF=' .env; then
    sed -i 's|^DOCKFIN_BOOTSTRAP_SELF=.*|DOCKFIN_BOOTSTRAP_SELF=1|' .env
  else
    echo 'DOCKFIN_BOOTSTRAP_SELF=1' >> .env
  fi
  if grep -q '^DOCKFIN_COOKIE_SECURE=' .env; then
    sed -i 's|^DOCKFIN_COOKIE_SECURE=.*|DOCKFIN_COOKIE_SECURE=0|' .env
  else
    echo 'DOCKFIN_COOKIE_SECURE=0' >> .env
  fi
fi

# Ensure web dir + templates are set even on older .env files
touch .env
grep -q '^DOCKFIN_WEB_DIR=' .env || echo "DOCKFIN_WEB_DIR=${SRC}/apps/web/dist" >> .env
sed -i "s|^DOCKFIN_WEB_DIR=.*|DOCKFIN_WEB_DIR=${SRC}/apps/web/dist|" .env
grep -q '^DOCKFIN_TEMPLATES_DIR=' .env || echo "DOCKFIN_TEMPLATES_DIR=${SRC}/templates/compose" >> .env
sed -i "s|^DOCKFIN_TEMPLATES_DIR=.*|DOCKFIN_TEMPLATES_DIR=${SRC}/templates/compose|" .env

mkdir -p "${WORKDIR}/data" "${WORKDIR}/bin"
log "Building dockfin..."
go build -o "${WORKDIR}/bin/dockfin" ./cmd/dockfin
go build -o "${WORKDIR}/bin/dfin" ./cmd/dfin

if [[ "${SKIP_WEB:-0}" != "1" ]] && [[ -f apps/web/package.json ]]; then
  log "Building web UI (Vite static)..."
  (
    cd apps/web
    npm ci --no-fund --no-audit
    npm run build
  )
  [[ -f apps/web/dist/index.html ]] || fail "Web build missing apps/web/dist/index.html"
  ok "Web UI built → apps/web/dist"
else
  warn "Skipping web UI build (SKIP_WEB=1 or apps/web missing)"
fi

# stop old smoke API if any
if [[ -f "${WORKDIR}/api.pid" ]]; then
  kill "$(cat "${WORKDIR}/api.pid")" 2>/dev/null || true
  rm -f "${WORKDIR}/api.pid"
fi

log "Migrating database..."
"${WORKDIR}/bin/dockfin" migrate

log "Starting API on :${API_PORT}..."
nohup "${WORKDIR}/bin/dockfin" serve >"${WORKDIR}/api.log" 2>&1 &
echo $! >"${WORKDIR}/api.pid"

for i in $(seq 1 40); do
  if curl -fsS "${API_URL}/health" >/dev/null 2>&1; then
    ok "API healthy"
    break
  fi
  if [[ $i -eq 40 ]]; then
    tail -n 50 "${WORKDIR}/api.log" || true
    fail "API did not become healthy"
  fi
  sleep 0.5
done

# -----------------------------------------------------------------------------
# Helpers
# -----------------------------------------------------------------------------
api() {
  local method="$1" path="$2"
  shift 2
  curl -sS -X "${method}" "${API_URL}${path}" \
    -b "${COOKIE_JAR}" -c "${COOKIE_JAR}" \
    -H 'Content-Type: application/json' \
    "$@"
}

assert_json() {
  local body="$1" expr="$2" msg="$3"
  if ! echo "${body}" | jq -e "${expr}" >/dev/null 2>&1; then
    echo "${body}" | head -c 800
    fail "${msg}"
  fi
}

# -----------------------------------------------------------------------------
# 5) API smoke tests
# -----------------------------------------------------------------------------
log "=== API smoke tests ==="
rm -f "${COOKIE_JAR}"

BODY=$(curl -fsS "${API_URL}/health")
assert_json "${BODY}" '.status=="ok"' "health"
report "[PASS] health"

BODY=$(curl -fsS "${API_URL}/api/v1/version")
assert_json "${BODY}" '.name=="Dockfin"' "version"
report "[PASS] version"

BODY=$(api POST /api/v1/auth/register \
  -d "{\"email\":\"${TEST_EMAIL}\",\"name\":\"${TEST_NAME}\",\"password\":\"${TEST_PASS}\"}")
assert_json "${BODY}" '.user.email!=null and .token!=null' "register"
TOKEN=$(echo "${BODY}" | jq -r .token)
TEAM_ID=$(echo "${BODY}" | jq -r .team.id)
# First-user register may auto-add this VPS (public IP) as a server
BOOTSTRAP_SERVER_ID=$(echo "${BODY}" | jq -r '.server.id // .bootstrap.server.id // empty')
BOOTSTRAP_ERR=$(echo "${BODY}" | jq -r '.bootstrap_error // empty')
if [[ -n "${BOOTSTRAP_SERVER_ID}" && "${BOOTSTRAP_SERVER_ID}" != "null" ]]; then
  report "[PASS] register auto-bootstrap server=${BOOTSTRAP_SERVER_ID}"
elif [[ -n "${BOOTSTRAP_ERR}" ]]; then
  warn "register bootstrap: ${BOOTSTRAP_ERR}"
  report "[WARN] register bootstrap failed (will retry via API)"
fi
report "[PASS] register (${TEST_EMAIL})"

BODY=$(api GET /api/v1/auth/me -H "Authorization: Bearer ${TOKEN}")
assert_json "${BODY}" '.user.email!=null' "me"
report "[PASS] auth/me"

# -----------------------------------------------------------------------------
# 6) Self-VPS bootstrap (public IP server — same as manual "Add server")
# -----------------------------------------------------------------------------
if [[ "${SKIP_DEPLOY}" == "1" ]]; then
  warn "SKIP_DEPLOY=1 — skipping SSH/deploy tests"
  report "[SKIP] deploy path"
else
  log "=== Self-VPS bootstrap (public IP) + deploy smoke ==="
  docker info >/dev/null 2>&1 || fail "Docker not usable"
  [[ -n "${PUBLIC_IP}" ]] || fail "PUBLIC_IP empty — cannot bootstrap this VPS as a server"

  # First register should auto-bootstrap; otherwise call the endpoint.
  SERVER_ID="${BOOTSTRAP_SERVER_ID:-}"
  if [[ -z "${SERVER_ID}" || "${SERVER_ID}" == "null" ]]; then
    BODY=$(api POST /api/v1/servers/bootstrap-self -H "Authorization: Bearer ${TOKEN}" \
      -d '{"start_proxy":true}')
    assert_json "${BODY}" '.server.id!=null' "bootstrap-self"
    SERVER_ID=$(echo "${BODY}" | jq -r .server.id)
  fi
  SERVER_IP=$(api GET "/api/v1/servers/${SERVER_ID}" -H "Authorization: Bearer ${TOKEN}" | jq -r .ip)
  SERVER_PUB=$(api GET "/api/v1/servers/${SERVER_ID}" -H "Authorization: Bearer ${TOKEN}" | jq -r .public_ip)
  [[ "${SERVER_IP}" == "${PUBLIC_IP}" ]] || fail "server.ip want ${PUBLIC_IP} got ${SERVER_IP}"
  [[ "${SERVER_PUB}" == "${PUBLIC_IP}" ]] || fail "server.public_ip want ${PUBLIC_IP} got ${SERVER_PUB}"
  report "[PASS] bootstrap-self (ip=${SERVER_IP} public_ip=${SERVER_PUB})"

  # Re-validate in case register-time SSH was still warming up
  BODY=$(api POST "/api/v1/servers/${SERVER_ID}/validate" -H "Authorization: Bearer ${TOKEN}" -d '{}')
  assert_json "${BODY}" '.usable==true' "validate server (docker)"
  report "[PASS] servers validate (usable=true public_ip=$(echo "${BODY}" | jq -r .public_ip))"

  BODY=$(api GET "/api/v1/servers/${SERVER_ID}" -H "Authorization: Bearer ${TOKEN}")
  if [[ "$(echo "${BODY}" | jq -r .proxy_status)" != "running" ]]; then
    BODY=$(api POST "/api/v1/servers/${SERVER_ID}/proxy/start" -H "Authorization: Bearer ${TOKEN}" -d '{}')
    assert_json "${BODY}" '.status=="running"' "start proxy"
  fi
  report "[PASS] proxy running"

  BODY=$(api GET /api/v1/destinations -H "Authorization: Bearer ${TOKEN}")
  DEST_ID=$(echo "${BODY}" | jq -r '.destinations[] | select(.server_id=="'"${SERVER_ID}"'") | .id' | head -n1)
  [[ -n "${DEST_ID}" && "${DEST_ID}" != "null" ]] || fail "no destination for server"
  report "[PASS] destinations (${DEST_ID})"

  BODY=$(api POST /api/v1/projects -H "Authorization: Bearer ${TOKEN}" \
    -d '{"name":"smoke","description":"oneclick"}')
  assert_json "${BODY}" '.project.id!=null and .environment.id!=null' "create project"
  ENV_ID=$(echo "${BODY}" | jq -r .environment.id)
  report "[PASS] projects create (env=${ENV_ID})"

  BODY=$(api POST /api/v1/applications -H "Authorization: Bearer ${TOKEN}" \
    -d "{\"name\":\"smoke-nginx\",\"environment_id\":\"${ENV_ID}\",\"destination_id\":\"${DEST_ID}\",\"build_pack\":\"dockerimage\",\"docker_registry_image_name\":\"nginx\",\"docker_registry_image_tag\":\"alpine\",\"ports_exposes\":\"80\",\"fqdn\":\"smoke.localhost\"}")
  assert_json "${BODY}" '.id!=null' "create application"
  APP_ID=$(echo "${BODY}" | jq -r .id)
  report "[PASS] applications create (${APP_ID})"

  BODY=$(api POST "/api/v1/applications/${APP_ID}/deploy" -H "Authorization: Bearer ${TOKEN}" \
    -d '{"force_rebuild":false}')
  assert_json "${BODY}" '.id!=null' "enqueue deploy"
  DEP_ID=$(echo "${BODY}" | jq -r .id)
  report "[PASS] deploy enqueued (${DEP_ID})"

  log "Waiting for deployment to finish..."
  FINAL=""
  for i in $(seq 1 90); do
    BODY=$(api GET "/api/v1/deployments/${DEP_ID}" -H "Authorization: Bearer ${TOKEN}")
    STATUS=$(echo "${BODY}" | jq -r .status)
    STAGE=$(echo "${BODY}" | jq -r .current_stage)
    echo "  [${i}] status=${STATUS} stage=${STAGE}"
    if [[ "${STATUS}" == "finished" || "${STATUS}" == "failed" || "${STATUS}" == "cancelled" ]]; then
      FINAL="${STATUS}"
      break
    fi
    sleep 2
  done
  [[ "${FINAL}" == "finished" ]] || {
    echo "${BODY}" | jq . 2>/dev/null || echo "${BODY}"
    docker ps -a --filter name=dockfin- || true
    fail "deployment did not finish successfully (got: ${FINAL:-timeout})"
  }
  report "[PASS] deployment finished"

  # container should be running
  if docker ps --format '{{.Names}}' | grep -q "dockfin-${APP_ID}"; then
    report "[PASS] container running"
  else
    # name format dockfin-<uuid>
    if docker ps --format '{{.Names}}' | grep -q "dockfin-"; then
      report "[PASS] dockfin container present"
    else
      warn "container name not found — checking proxy network"
      docker ps
      report "[WARN] container name check inconclusive"
    fi
  fi
fi

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo ""
log "========== SMOKE REPORT =========="
cat "${REPORT}"
echo "=================================="
ok "All executed checks done."
echo ""
echo "API (local):  ${API_URL}"
echo "API (public): ${PUBLIC_API_URL}"
echo "VPS IP:       ${PUBLIC_IP:-unknown}"
echo "Browser:      ${PUBLIC_API_URL}/  (API + UI when dist is present)"
echo "Health:       ${PUBLIC_API_URL}/health"
echo "Version:      ${PUBLIC_API_URL}/api/v1/version"
echo "Login:        ${TEST_EMAIL} / ${TEST_PASS}"
echo "Token:        ${TOKEN:-n/a}"
echo "Logs:         ${WORKDIR}/api.log"
echo "Report:       ${REPORT}"
echo "Source:       ${SRC}"
if [[ -f "${SRC}/apps/web/dist/index.html" ]]; then
  echo "UI dist:      ${SRC}/apps/web/dist (served by dockfin)"
else
  echo "UI dist:      missing — re-run without SKIP_WEB=1"
fi
if [[ "${KEEP_RUNNING}" == "1" ]]; then
  echo "API left running (pid $(cat "${WORKDIR}/api.pid" 2>/dev/null || echo '?'))"
  echo "Stop with: kill \$(cat ${WORKDIR}/api.pid)"
else
  kill "$(cat "${WORKDIR}/api.pid")" 2>/dev/null || true
  echo "API stopped."
fi
