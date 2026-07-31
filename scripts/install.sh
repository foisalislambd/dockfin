#!/usr/bin/env bash
set -euo pipefail

# Goolify — production install (Ubuntu/Debian)
#
#   curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash
#
# Pulls the published image from GitHub Container Registry (GHCR), then starts
# Postgres + Goolify via Docker Compose. No manual .env editing.
#
# Optional:
#   GOOLIFY_VERSION=1.0.9          # pin tag (default: latest)
#   GOOLIFY_IMAGE=ghcr.io/...:...  # full override
#   GOOLIFY_DIR=/data/goolify      # install directory

GOOLIFY_DIR="${GOOLIFY_DIR:-/data/goolify}"
COMPOSE_FILE="${GOOLIFY_DIR}/docker-compose.yml"
ENV_FILE="${GOOLIFY_DIR}/.env"

# Production default: GitHub Packages (GHCR)
GHCR_IMAGE="ghcr.io/foisalislambd/goolify"
VERSION="${GOOLIFY_VERSION:-latest}"
IMAGE="${GOOLIFY_IMAGE:-${GHCR_IMAGE}:${VERSION}}"

echo "==> Goolify production installer"
echo "    Install dir: ${GOOLIFY_DIR}"
echo "    Image:       ${IMAGE}"
echo "    Registry:    GitHub Container Registry (ghcr.io)"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root:"
  echo "  curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash"
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "==> Installing Docker…"
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose plugin required (install docker-compose-plugin)."
  exit 1
fi

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
GOOLIFY_ENV=production
GOOLIFY_HTTP_ADDR=:8000
GOOLIFY_DATABASE_URL=postgres://goolify:${DB_PASS}@postgres:5432/goolify?sslmode=disable
GOOLIFY_MASTER_KEY=${MASTER_KEY}
GOOLIFY_CORS_ORIGINS=*
GOOLIFY_PUBLIC_URL=http://${PUBLIC_IP}:8000
GOOLIFY_PUBLIC_IP=${PUBLIC_IP}
GOOLIFY_BOOTSTRAP_SELF=1
GOOLIFY_DATA_DIR=/data
GOOLIFY_TEMPLATES_DIR=/app/templates
GOOLIFY_WEB_DIR=/app/web
POSTGRES_PASSWORD=${DB_PASS}
EOF
  chmod 600 "${ENV_FILE}"
  echo "==> Generated secure secrets"
else
  if ! grep -q '^POSTGRES_PASSWORD=' "${ENV_FILE}"; then
    DBURL=$(grep '^GOOLIFY_DATABASE_URL=' "${ENV_FILE}" | cut -d= -f2-)
    DBPASS=$(echo "$DBURL" | sed -n 's#.*goolify:\([^@]*\)@.*#\1#p')
    if [[ -n "${DBPASS}" ]]; then
      echo "POSTGRES_PASSWORD=${DBPASS}" >> "${ENV_FILE}"
    fi
  fi
  # Ensure production env flag on existing installs
  if grep -q '^GOOLIFY_ENV=' "${ENV_FILE}"; then
    sed -i 's/^GOOLIFY_ENV=.*/GOOLIFY_ENV=production/' "${ENV_FILE}"
  else
    echo "GOOLIFY_ENV=production" >> "${ENV_FILE}"
  fi
  echo "==> Using existing ${ENV_FILE}"
fi

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
    pull_policy: always
    env_file: .env
    environment:
      GOOLIFY_HTTP_ADDR: ":8000"
    ports:
      - "8000:8000"
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

echo "==> Pulling ${IMAGE}…"
# Avoid host-exported GOOLIFY_* overriding compose/.env (breaks DB hostname).
unset GOOLIFY_DATABASE_URL GOOLIFY_MASTER_KEY GOOLIFY_HTTP_ADDR GOOLIFY_PUBLIC_URL 2>/dev/null || true

if ! docker compose pull; then
  echo ""
  echo "Failed to pull ${IMAGE}"
  echo "If the GHCR package is private, make it public under:"
  echo "  https://github.com/users/foisalislambd/packages/container/goolify/settings"
  echo "Or login first:  echo \$GHCR_TOKEN | docker login ghcr.io -u USERNAME --password-stdin"
  exit 1
fi

echo "==> Starting Goolify…"
# Recreate so a newly pulled digest actually replaces a running container.
docker compose up -d --force-recreate --remove-orphans

echo "==> Waiting for health…"
ok=0
for i in $(seq 1 60); do
  if curl -fsS --max-time 2 "http://127.0.0.1:8000/health" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 2
done

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
IP="${IP:-$PUBLIC_IP}"

echo ""
if [[ "${ok}" -eq 1 ]]; then
  echo "Goolify is ready (production)."
else
  echo "Containers started; health check still warming up."
  echo "Check: cd ${GOOLIFY_DIR} && docker compose logs -f goolify"
fi
echo ""
echo "  Dashboard: http://${IP}:8000/"
echo "  Health:    http://${IP}:8000/health"
echo "  Version:   http://${IP}:8000/api/v1/version"
echo "  Data:      ${GOOLIFY_DIR}"
echo "  Image:     ${IMAGE}"
echo ""
echo "Next: open the URL → Register your admin account."
echo "First register auto-adds this VPS as a server. Then create a project and deploy."
echo ""
echo "Update later:"
echo "  cd ${GOOLIFY_DIR} && docker compose pull && docker compose up -d"
echo ""
echo "Pin a version:"
echo "  sudo GOOLIFY_VERSION=1.0.9 bash -c 'curl -fsSL …/install.sh | bash'"
echo ""
