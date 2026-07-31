#!/usr/bin/env bash
set -euo pipefail

# Goolify — local / development install
#
# Builds a local Docker image from this repo and runs compose (no registry pull).
# Uses the same install dir as production by default so rebuilds keep your DB.
# For production servers use scripts/install.sh instead (pulls from GHCR).
#
# Usage (from repo root):
#   sudo bash scripts/install-dev.sh
#
# Optional:
#   GOOLIFY_DIR=/data/goolify      # same as production (default)
#   GOOLIFY_IMAGE=goolify:local
#   GOOLIFY_VERSION=dev
#   GOOLIFY_HOST_PORT=8000         # host port published to the panel

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

GOOLIFY_DIR="${GOOLIFY_DIR:-/data/goolify}"
COMPOSE_FILE="${GOOLIFY_DIR}/docker-compose.yml"
ENV_FILE="${GOOLIFY_DIR}/.env"
IMAGE="${GOOLIFY_IMAGE:-goolify:local}"
VERSION="${GOOLIFY_VERSION:-dev}"
HOST_PORT="${GOOLIFY_HOST_PORT:-8000}"

echo "==> Goolify development installer"
echo "    Repo:        ${REPO_ROOT}"
echo "    Install dir: ${GOOLIFY_DIR}"
echo "    Image:       ${IMAGE} (built locally — no pull)"
echo "    Port:        ${HOST_PORT}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo bash scripts/install-dev.sh"
  exit 1
fi

if [[ ! -f "${REPO_ROOT}/deploy/docker/Dockerfile.api" ]]; then
  echo "Dockerfile not found. Run this script from a goolify git checkout:"
  echo "  cd /path/to/goolify && sudo bash scripts/install-dev.sh"
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

mkdir -p "${GOOLIFY_DIR}"
cd "${GOOLIFY_DIR}"

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
GOOLIFY_ENV=development
GOOLIFY_HTTP_ADDR=:8000
GOOLIFY_DATABASE_URL=postgres://goolify:${DB_PASS}@postgres:5432/goolify?sslmode=disable
GOOLIFY_MASTER_KEY=${MASTER_KEY}
GOOLIFY_CORS_ORIGINS=*
GOOLIFY_PUBLIC_URL=http://${PUBLIC_IP}:${HOST_PORT}
GOOLIFY_PUBLIC_IP=${PUBLIC_IP}
GOOLIFY_BOOTSTRAP_SELF=1
GOOLIFY_DATA_DIR=/data
GOOLIFY_TEMPLATES_DIR=/app/templates
GOOLIFY_WEB_DIR=/app/web
POSTGRES_PASSWORD=${DB_PASS}
EOF
  chmod 600 "${ENV_FILE}"
  echo "==> Generated .env for development"
else
  if ! grep -q '^POSTGRES_PASSWORD=' "${ENV_FILE}"; then
    DBURL=$(grep '^GOOLIFY_DATABASE_URL=' "${ENV_FILE}" | cut -d= -f2-)
    DBPASS=$(echo "$DBURL" | sed -n 's#.*goolify:\([^@]*\)@.*#\1#p')
    if [[ -n "${DBPASS}" ]]; then
      echo "POSTGRES_PASSWORD=${DBPASS}" >> "${ENV_FILE}"
    fi
  fi
  if grep -q '^GOOLIFY_ENV=' "${ENV_FILE}"; then
    sed -i 's/^GOOLIFY_ENV=.*/GOOLIFY_ENV=development/' "${ENV_FILE}"
  else
    echo "GOOLIFY_ENV=development" >> "${ENV_FILE}"
  fi
  echo "==> Using existing ${ENV_FILE}"
fi

# Same volume names as production so switching install.sh ↔ install-dev.sh keeps DB/data.
cat > "${COMPOSE_FILE}" <<EOF
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: goolify
      POSTGRES_PASSWORD: \${POSTGRES_PASSWORD:-goolify}
      POSTGRES_DB: goolify
    volumes:
      - goolify-pg:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U goolify"]
      interval: 5s
      timeout: 5s
      retries: 30
    restart: unless-stopped

  goolify:
    image: ${IMAGE}
    pull_policy: never
    env_file: .env
    environment:
      GOOLIFY_HTTP_ADDR: ":8000"
    ports:
      - "${HOST_PORT}:8000"
    volumes:
      - goolify-data:/data
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

volumes:
  goolify-pg:
  goolify-data:
EOF

echo "==> Starting (local image, skip pull)…"
unset GOOLIFY_DATABASE_URL GOOLIFY_MASTER_KEY GOOLIFY_HTTP_ADDR GOOLIFY_PUBLIC_URL 2>/dev/null || true
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
  echo "Goolify is ready (development)."
else
  echo "Containers started; check logs:"
  echo "  cd ${GOOLIFY_DIR} && docker compose logs -f goolify"
fi
echo ""
echo "  Dashboard: http://${IP}:${HOST_PORT}/"
echo "  Health:    http://${IP}:${HOST_PORT}/health"
echo "  Data:      ${GOOLIFY_DIR}"
echo "  Image:     ${IMAGE}"
echo ""
echo "Rebuild after code changes:"
echo "  sudo bash ${SCRIPT_DIR}/install-dev.sh"
echo ""
echo "Switch to production image (GHCR):"
echo "  curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash"
echo ""
