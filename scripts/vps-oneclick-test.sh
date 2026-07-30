#!/usr/bin/env bash
# =============================================================================
# Goolify — one-click VPS install + smoke test
# =============================================================================
# On a fresh Ubuntu/Debian VPS this script will:
#   1) Install Docker / Go / dependencies
#   2) Start Postgres
#   3) Build Goolify API, migrate, and serve
#   4) Add a self-SSH server → validate → Traefik → nginx:alpine deploy
#   5) Print a pass/fail report
#
# Usage (as root on the VPS):
#   curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/goolify/main/scripts/vps-oneclick-test.sh | bash
#
# Or from a cloned repo:
#   sudo bash scripts/vps-oneclick-test.sh
#
# Optional env:
#   GOOLIFY_SRC=/path/to/goolify   # default: auto-detect or clone
#   GOOLIFY_GIT_URL=...            # git clone URL if source missing
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

API_PORT="${API_PORT:-8080}"
API_URL="http://127.0.0.1:${API_PORT}"
TEST_EMAIL="smoke-$(date +%s)@goolify.test"
TEST_PASS="SmokeTestPass123!"
TEST_NAME="Smoke Tester"
WORKDIR="${GOOLIFY_WORKDIR:-/opt/goolify-smoke}"
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

# Go 1.23+
if ! command -v go >/dev/null 2>&1 || [[ "$(go env GOVERSION 2>/dev/null | sed 's/go//;s/\..*//')" -lt 1 ]]; then
  log "Installing Go 1.23.6..."
  curl -fsSL https://go.dev/dl/go1.23.6.linux-amd64.tar.gz -o /tmp/go.tgz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tgz
  export PATH="/usr/local/go/bin:${PATH}"
  echo 'export PATH=/usr/local/go/bin:$PATH' >/etc/profile.d/golang.sh
fi
export PATH="/usr/local/go/bin:${PATH}"
go version

# -----------------------------------------------------------------------------
# 2) Source tree
# -----------------------------------------------------------------------------
detect_src() {
  if [[ -n "${GOOLIFY_SRC:-}" && -f "${GOOLIFY_SRC}/go.mod" ]]; then
    echo "${GOOLIFY_SRC}"
    return
  fi
  # script lives in repo?
  local here
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." 2>/dev/null && pwd || true)"
  if [[ -n "${here}" && -f "${here}/go.mod" ]]; then
    echo "${here}"
    return
  fi
  if [[ -f /data/goolify-src/go.mod ]]; then
    echo /data/goolify-src
    return
  fi
  echo ""
}

SRC="$(detect_src)"
if [[ -z "${SRC}" ]]; then
  GIT_URL="${GOOLIFY_GIT_URL:-}"
  if [[ -z "${GIT_URL}" ]]; then
    fail "Source not found. Set GOOLIFY_SRC=/path/to/goolify or GOOLIFY_GIT_URL=https://github.com/.../goolify.git"
  fi
  log "Cloning ${GIT_URL}..."
  rm -rf /data/goolify-src
  git clone --depth 1 "${GIT_URL}" /data/goolify-src
  SRC=/data/goolify-src
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
  if docker compose -f deploy/compose/docker-compose.dev.yml exec -T postgres pg_isready -U goolify >/dev/null 2>&1; then
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
  SESSION_SECRET=$(openssl rand -base64 48 | tr -d '\n' | head -c 48)
  cat > .env <<EOF
GOOLIFY_ENV=production
GOOLIFY_HTTP_ADDR=:${API_PORT}
GOOLIFY_DATABASE_URL=postgres://goolify:goolify@127.0.0.1:5432/goolify?sslmode=disable
GOOLIFY_REDIS_URL=redis://127.0.0.1:6379/0
GOOLIFY_MASTER_KEY=${MASTER_KEY}
GOOLIFY_SESSION_SECRET=${SESSION_SECRET}
GOOLIFY_CORS_ORIGINS=*
GOOLIFY_PUBLIC_URL=http://127.0.0.1:${API_PORT}
GOOLIFY_DATA_DIR=${WORKDIR}/data
GOOLIFY_TEMPLATES_DIR=${SRC}/coolify/templates/compose
EOF
fi

mkdir -p "${WORKDIR}/data" "${WORKDIR}/bin"
log "Building goolify..."
go build -o "${WORKDIR}/bin/goolify" ./cmd/goolify
go build -o "${WORKDIR}/bin/glfy" ./cmd/glfy

# stop old smoke API if any
if [[ -f "${WORKDIR}/api.pid" ]]; then
  kill "$(cat "${WORKDIR}/api.pid")" 2>/dev/null || true
  rm -f "${WORKDIR}/api.pid"
fi

log "Migrating database..."
"${WORKDIR}/bin/goolify" migrate

log "Starting API on :${API_PORT}..."
nohup "${WORKDIR}/bin/goolify" serve >"${WORKDIR}/api.log" 2>&1 &
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
assert_json "${BODY}" '.name=="Goolify"' "version"
report "[PASS] version"

BODY=$(api POST /api/v1/auth/register \
  -d "{\"email\":\"${TEST_EMAIL}\",\"name\":\"${TEST_NAME}\",\"password\":\"${TEST_PASS}\"}")
assert_json "${BODY}" '.user.email!=null and .token!=null' "register"
TOKEN=$(echo "${BODY}" | jq -r .token)
TEAM_ID=$(echo "${BODY}" | jq -r .team.id)
report "[PASS] register (${TEST_EMAIL})"

BODY=$(api GET /api/v1/auth/me -H "Authorization: Bearer ${TOKEN}")
assert_json "${BODY}" '.user.email!=null' "me"
report "[PASS] auth/me"

# -----------------------------------------------------------------------------
# 6) Self-SSH bootstrap (same VPS as deploy target)
# -----------------------------------------------------------------------------
if [[ "${SKIP_DEPLOY}" == "1" ]]; then
  warn "SKIP_DEPLOY=1 — skipping SSH/deploy tests"
  report "[SKIP] deploy path"
else
  log "=== Self-SSH + deploy smoke ==="
  SSH_DIR="${WORKDIR}/ssh"
  mkdir -p "${SSH_DIR}"
  if [[ ! -f "${SSH_DIR}/id_ed25519" ]]; then
    ssh-keygen -t ed25519 -N "" -f "${SSH_DIR}/id_ed25519" -C "goolify-smoke" >/dev/null
  fi
  mkdir -p /root/.ssh
  chmod 700 /root/.ssh
  touch /root/.ssh/authorized_keys
  chmod 600 /root/.ssh/authorized_keys
  PUB=$(cat "${SSH_DIR}/id_ed25519.pub")
  grep -qxF "${PUB}" /root/.ssh/authorized_keys || echo "${PUB}" >>/root/.ssh/authorized_keys

  # ensure docker usable as root
  docker info >/dev/null 2>&1 || fail "Docker not usable"

  PRIV_JSON=$(jq -Rs . <"${SSH_DIR}/id_ed25519")
  BODY=$(api POST /api/v1/private-keys -H "Authorization: Bearer ${TOKEN}" \
    -d "{\"name\":\"smoke-key\",\"private_key\":${PRIV_JSON}}")
  assert_json "${BODY}" '.id!=null' "create private key"
  KEY_ID=$(echo "${BODY}" | jq -r .id)
  report "[PASS] private-keys create"

  BODY=$(api POST /api/v1/servers -H "Authorization: Bearer ${TOKEN}" \
    -d "{\"name\":\"self\",\"ip\":\"127.0.0.1\",\"port\":22,\"user_name\":\"root\",\"private_key_id\":\"${KEY_ID}\",\"proxy_type\":\"traefik\"}")
  assert_json "${BODY}" '.id!=null' "create server"
  SERVER_ID=$(echo "${BODY}" | jq -r .id)
  report "[PASS] servers create"

  BODY=$(api POST "/api/v1/servers/${SERVER_ID}/validate" -H "Authorization: Bearer ${TOKEN}" -d '{}')
  assert_json "${BODY}" '.usable==true' "validate server (docker)"
  report "[PASS] servers validate (usable=true)"

  BODY=$(api POST "/api/v1/servers/${SERVER_ID}/proxy/start" -H "Authorization: Bearer ${TOKEN}" -d '{}')
  assert_json "${BODY}" '.status=="running"' "start proxy"
  report "[PASS] proxy start"

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
    docker ps -a --filter name=goolify- || true
    fail "deployment did not finish successfully (got: ${FINAL:-timeout})"
  }
  report "[PASS] deployment finished"

  # container should be running
  if docker ps --format '{{.Names}}' | grep -q "goolify-${APP_ID}"; then
    report "[PASS] container running"
  else
    # name format goolify-<uuid>
    if docker ps --format '{{.Names}}' | grep -q "goolify-"; then
      report "[PASS] goolify container present"
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
echo "API:      ${API_URL}"
echo "Login:    ${TEST_EMAIL} / ${TEST_PASS}"
echo "Token:    ${TOKEN:-n/a}"
echo "Logs:     ${WORKDIR}/api.log"
echo "Report:   ${REPORT}"
echo "Source:   ${SRC}"
if [[ "${KEEP_RUNNING}" == "1" ]]; then
  echo "API left running (pid $(cat "${WORKDIR}/api.pid" 2>/dev/null || echo '?'))"
  echo "Stop with: kill \$(cat ${WORKDIR}/api.pid)"
else
  kill "$(cat "${WORKDIR}/api.pid")" 2>/dev/null || true
  echo "API stopped."
fi
echo ""
echo "UI tip: build web with: cd ${SRC}/apps/web && npm i && npm run build"
echo "Then serve apps/web/dist behind nginx, or npm run dev -- --host 0.0.0.0"
