#!/usr/bin/env bash
set -euo pipefail

# Dockfin — local / development install
#
# Builds a local Docker image from this repo and runs compose (no registry pull).
# Uses the same install dir as production by default so rebuilds keep your DB.
# For production servers use scripts/install.sh instead (pulls from GHCR).
#
# Usage (from repo root):
#   sudo bash scripts/install-dev.sh
#
# Optional:
#   DOCKFIN_DIR=/data/dockfin      # same as production (default)
#   DOCKFIN_IMAGE=dockfin:local
#   DOCKFIN_VERSION=dev
#   DOCKFIN_HOST_PORT=8000         # host port published to the panel

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

DOCKFIN_DIR="${DOCKFIN_DIR:-/data/dockfin}"
COMPOSE_FILE="${DOCKFIN_DIR}/docker-compose.yml"
ENV_FILE="${DOCKFIN_DIR}/.env"
IMAGE="${DOCKFIN_IMAGE:-dockfin:local}"
VERSION="${DOCKFIN_VERSION:-dev}"
HOST_PORT="${DOCKFIN_HOST_PORT:-8000}"

echo "==> Dockfin development installer"
echo "    Repo:        ${REPO_ROOT}"
echo "    Install dir: ${DOCKFIN_DIR}"
echo "    Image:       ${IMAGE} (built locally — no pull)"
echo "    Port:        ${HOST_PORT}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo bash scripts/install-dev.sh"
  exit 1
fi

if [[ ! -f "${REPO_ROOT}/deploy/docker/Dockerfile.api" ]]; then
  echo "Dockerfile not found. Run this script from a dockfin git checkout:"
  echo "  cd /path/to/dockfin && sudo bash scripts/install-dev.sh"
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "==> Installing Docker…"
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose plugin required."
  exit 1
fi

echo "==> Building ${IMAGE} from source…"
docker build \
  -f "${REPO_ROOT}/deploy/docker/Dockerfile.api" \
  --build-arg "VERSION=${VERSION}" \
  -t "${IMAGE}" \
  "${REPO_ROOT}"

mkdir -p "${DOCKFIN_DIR}"
cd "${DOCKFIN_DIR}"

PUBLIC_IP="$(curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null \
  || curl -4 -fsS --max-time 5 https://ifconfig.me/ip 2>/dev/null \
  || hostname -I 2>/dev/null | awk '{print $1}')"
PUBLIC_IP="$(echo "${PUBLIC_IP}" | tr -d '[:space:]')"
if [[ -z "${PUBLIC_IP}" ]]; then
  PUBLIC_IP="127.0.0.1"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  MASTER_KEY=$(openssl rand -base64 48 | tr -d '\n' | head -c 48)
  DB_PASS=$(openssl rand -base64 24 | tr -d '\n=/+' | head -c 24)
  cat > "${ENV_FILE}" <<EOF
DOCKFIN_ENV=development
DOCKFIN_HTTP_ADDR=:8000
DOCKFIN_DATABASE_URL=postgres://dockfin:${DB_PASS}@postgres:5432/dockfin?sslmode=disable
DOCKFIN_MASTER_KEY=${MASTER_KEY}
DOCKFIN_CORS_ORIGINS=http://${PUBLIC_IP}:${HOST_PORT}
DOCKFIN_PUBLIC_URL=http://${PUBLIC_IP}:${HOST_PORT}
DOCKFIN_PUBLIC_IP=${PUBLIC_IP}
DOCKFIN_BOOTSTRAP_SELF=1
DOCKFIN_DATA_DIR=/data
DOCKFIN_TEMPLATES_DIR=/app/templates
DOCKFIN_WEB_DIR=/app/web
POSTGRES_PASSWORD=${DB_PASS}
EOF
  chmod 600 "${ENV_FILE}"
  echo "==> Generated .env for development"
else
  if ! grep -q '^POSTGRES_PASSWORD=' "${ENV_FILE}"; then
    DBURL=$(grep '^DOCKFIN_DATABASE_URL=' "${ENV_FILE}" | cut -d= -f2-)
    DBPASS=$(echo "$DBURL" | sed -n 's#.*dockfin:\([^@]*\)@.*#\1#p')
    if [[ -n "${DBPASS}" ]]; then
      echo "POSTGRES_PASSWORD=${DBPASS}" >> "${ENV_FILE}"
    fi
  fi
  if grep -q '^DOCKFIN_ENV=' "${ENV_FILE}"; then
    sed -i 's/^DOCKFIN_ENV=.*/DOCKFIN_ENV=development/' "${ENV_FILE}"
  else
    echo "DOCKFIN_ENV=development" >> "${ENV_FILE}"
  fi
  echo "==> Using existing ${ENV_FILE}"
fi

echo "==> Ensuring host SSH dir for bootstrap (first-user auto-add server)…"
SSH_USER_HOME="/root"
mkdir -p "${SSH_USER_HOME}/.ssh"
chmod 700 "${SSH_USER_HOME}/.ssh"
touch "${SSH_USER_HOME}/.ssh/authorized_keys"
chmod 600 "${SSH_USER_HOME}/.ssh/authorized_keys"

echo "==> Ensuring shared Docker network (Traefik ↔ panel)…"
docker network create dockfin >/dev/null 2>&1 || true
mkdir -p /data/dockfin/proxy/traefik/dynamic /data/dockfin/proxy/traefik/letsencrypt

# Same volume names as production so switching install.sh ↔ install-dev.sh keeps DB/data.
# Mount host root .ssh so AuthorizePublicKey writes the bootstrap key on the VPS
# (container filesystem alone cannot SSH back to the host → "No Docker").
# Join external "dockfin" network so Traefik can route Settings Domain → panel :8000.
cat > "${COMPOSE_FILE}" <<EOF
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: dockfin
      POSTGRES_PASSWORD: \${POSTGRES_PASSWORD:-dockfin}
      POSTGRES_DB: dockfin
    volumes:
      - dockfin-pg:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U dockfin"]
      interval: 5s
      timeout: 5s
      retries: 30
    restart: unless-stopped
    networks:
      - default

  dockfin:
    image: ${IMAGE}
    pull_policy: never
    env_file: .env
    environment:
      DOCKFIN_HTTP_ADDR: ":8000"
      # Auto-update runs "docker compose pull/up -d" against the install dir.
      DOCKFIN_DIR: /host/dockfin
    ports:
      - "${HOST_PORT}:8000"
    volumes:
      - dockfin-data:/data
      - ${SSH_USER_HOME}/.ssh:/root/.ssh
      - /var/run/docker.sock:/var/run/docker.sock
      - ${DOCKFIN_DIR}:/host/dockfin
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped
    networks:
      default:
        aliases: [dockfin]
      dockfin:
        aliases: [dockfin]

networks:
  dockfin:
    external: true
    name: dockfin

volumes:
  dockfin-pg:
  dockfin-data:
EOF

echo "==> Starting (local image, skip pull)…"
unset DOCKFIN_DATABASE_URL DOCKFIN_MASTER_KEY DOCKFIN_HTTP_ADDR DOCKFIN_PUBLIC_URL 2>/dev/null || true
docker compose up -d --pull never --force-recreate --remove-orphans

echo "==> Waiting for health…"
ok=0
for i in $(seq 1 60); do
  if curl -fsS --max-time 2 "http://127.0.0.1:${HOST_PORT}/health" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 2
done

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
IP="${IP:-$PUBLIC_IP}"

echo ""
if [[ "${ok}" -eq 1 ]]; then
  echo "Dockfin is ready (development)."
else
  echo "Containers started; check logs:"
  echo "  cd ${DOCKFIN_DIR} && docker compose logs -f dockfin"
fi
echo ""
echo "  Dashboard: http://${IP}:${HOST_PORT}/"
echo "  Health:    http://${IP}:${HOST_PORT}/health"
echo "  Data:      ${DOCKFIN_DIR}"
echo "  Image:     ${IMAGE}"
echo ""
echo "Rebuild after code changes:"
echo "  sudo bash ${SCRIPT_DIR}/install-dev.sh"
echo ""
echo "Switch to production image (GHCR):"
echo "  curl -fsSL https://raw.githubusercontent.com/foisalislambd/dockfin/main/scripts/install.sh | sudo bash"
echo ""
