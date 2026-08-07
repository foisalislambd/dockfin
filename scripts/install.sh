#!/usr/bin/env bash
set -euo pipefail

# Dockfin — production install (Ubuntu/Debian)
#
#   curl -fsSL https://raw.githubusercontent.com/foisalislambd/dockfin/main/scripts/install.sh | sudo bash
#
# Pulls the published image from GitHub Container Registry (GHCR), then starts
# Postgres + Dockfin via Docker Compose. No manual .env editing.
#
# Optional:
#   DOCKFIN_VERSION=1.0.9          # pin tag (default: latest)
#   DOCKFIN_IMAGE=ghcr.io/...:...  # full override
#   DOCKFIN_DIR=/data/dockfin      # install directory

DOCKFIN_DIR="${DOCKFIN_DIR:-/data/dockfin}"
COMPOSE_FILE="${DOCKFIN_DIR}/docker-compose.yml"
ENV_FILE="${DOCKFIN_DIR}/.env"

# Production default: GitHub Packages (GHCR)
GHCR_IMAGE="ghcr.io/foisalislambd/dockfin"
VERSION="${DOCKFIN_VERSION:-latest}"
IMAGE="${DOCKFIN_IMAGE:-${GHCR_IMAGE}:${VERSION}}"

echo "==> Dockfin production installer"
echo "    Install dir: ${DOCKFIN_DIR}"
echo "    Image:       ${IMAGE}"
echo "    Registry:    GitHub Container Registry (ghcr.io)"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root:"
  echo "  curl -fsSL https://raw.githubusercontent.com/foisalislambd/dockfin/main/scripts/install.sh | sudo bash"
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
DOCKFIN_ENV=production
DOCKFIN_HTTP_ADDR=:8000
DOCKFIN_DATABASE_URL=postgres://dockfin:${DB_PASS}@postgres:5432/dockfin?sslmode=disable
DOCKFIN_MASTER_KEY=${MASTER_KEY}
DOCKFIN_CORS_ORIGINS=*
DOCKFIN_PUBLIC_URL=http://${PUBLIC_IP}:8000
DOCKFIN_PUBLIC_IP=${PUBLIC_IP}
DOCKFIN_BOOTSTRAP_SELF=1
DOCKFIN_DATA_DIR=/data
DOCKFIN_TEMPLATES_DIR=/app/templates
DOCKFIN_WEB_DIR=/app/web
POSTGRES_PASSWORD=${DB_PASS}
EOF
  chmod 600 "${ENV_FILE}"
  echo "==> Generated secure secrets"
else
  if ! grep -q '^POSTGRES_PASSWORD=' "${ENV_FILE}"; then
    DBURL=$(grep '^DOCKFIN_DATABASE_URL=' "${ENV_FILE}" | cut -d= -f2-)
    DBPASS=$(echo "$DBURL" | sed -n 's#.*dockfin:\([^@]*\)@.*#\1#p')
    if [[ -n "${DBPASS}" ]]; then
      echo "POSTGRES_PASSWORD=${DBPASS}" >> "${ENV_FILE}"
    fi
  fi
  # Ensure production env flag on existing installs
  if grep -q '^DOCKFIN_ENV=' "${ENV_FILE}"; then
    sed -i 's/^DOCKFIN_ENV=.*/DOCKFIN_ENV=production/' "${ENV_FILE}"
  else
    echo "DOCKFIN_ENV=production" >> "${ENV_FILE}"
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
    pull_policy: always
    env_file: .env
    environment:
      DOCKFIN_HTTP_ADDR: ":8000"
    ports:
      - "8000:8000"
    volumes:
      - dockfin-data:/data
      - ${SSH_USER_HOME}/.ssh:/root/.ssh
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

echo "==> Pulling ${IMAGE}…"
# Avoid host-exported DOCKFIN_* overriding compose/.env (breaks DB hostname).
unset DOCKFIN_DATABASE_URL DOCKFIN_MASTER_KEY DOCKFIN_HTTP_ADDR DOCKFIN_PUBLIC_URL 2>/dev/null || true

if ! docker compose pull; then
  echo ""
  echo "Failed to pull ${IMAGE}"
  echo "If the GHCR package is private, make it public under:"
  echo "  https://github.com/users/foisalislambd/packages/container/dockfin/settings"
  echo "Or login first:  echo \$GHCR_TOKEN | docker login ghcr.io -u USERNAME --password-stdin"
  exit 1
fi

echo "==> Starting Dockfin…"
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
  echo "Dockfin is ready (production)."
else
  echo "Containers started; health check still warming up."
  echo "Check: cd ${DOCKFIN_DIR} && docker compose logs -f dockfin"
fi
echo ""
echo "  Dashboard: http://${IP}:8000/"
echo "  Health:    http://${IP}:8000/health"
echo "  Version:   http://${IP}:8000/api/v1/version"
echo "  Data:      ${DOCKFIN_DIR}"
echo "  Image:     ${IMAGE}"
echo ""
echo "Next: open the URL → Register your admin account."
echo "First register auto-adds this VPS as a server. Then create a project and deploy."
echo ""
echo "Update later:"
echo "  cd ${DOCKFIN_DIR} && docker compose pull && docker compose up -d"
echo ""
echo "Pin a version:"
echo "  sudo DOCKFIN_VERSION=1.0.9 bash -c 'curl -fsSL …/install.sh | bash'"
echo ""
