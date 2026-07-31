#!/usr/bin/env bash
set -euo pipefail

# Goolify production installer (Ubuntu/Debian)
# Usage: curl -fsSL .../install.sh | bash

GOOLIFY_DIR="${GOOLIFY_DIR:-/data/goolify}"
COMPOSE_FILE="${GOOLIFY_DIR}/docker-compose.yml"
ENV_FILE="${GOOLIFY_DIR}/.env"

echo "==> Goolify installer"
echo "    Install dir: ${GOOLIFY_DIR}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root (sudo)."
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "==> Installing Docker..."
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose plugin required."
  exit 1
fi

mkdir -p "${GOOLIFY_DIR}/data"

if [[ ! -f "${ENV_FILE}" ]]; then
  MASTER_KEY=$(openssl rand -base64 48 | tr -d '\n' | head -c 48)
  DB_PASS=$(openssl rand -base64 24 | tr -d '\n=/+' | head -c 24)
  PUBLIC_IP="$(curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null || curl -4 -fsS --max-time 5 https://ifconfig.me/ip 2>/dev/null || hostname -I | awk '{print $1}')"
  PUBLIC_IP="$(echo "${PUBLIC_IP}" | tr -d '[:space:]')"
  cat > "${ENV_FILE}" <<EOF
GOOLIFY_ENV=production
GOOLIFY_HTTP_ADDR=:8000
GOOLIFY_DATABASE_URL=postgres://goolify:${DB_PASS}@postgres:5432/goolify?sslmode=disable
GOOLIFY_MASTER_KEY=${MASTER_KEY}
GOOLIFY_CORS_ORIGINS=*
GOOLIFY_PUBLIC_URL=http://${PUBLIC_IP}
GOOLIFY_PUBLIC_IP=${PUBLIC_IP}
GOOLIFY_BOOTSTRAP_SELF=1
GOOLIFY_DATA_DIR=/data
GOOLIFY_TEMPLATES_DIR=/app/templates
GOOLIFY_WEB_DIR=/app/web
EOF
  echo "==> Wrote ${ENV_FILE} (public IP: ${PUBLIC_IP})"
fi

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  cat > "${COMPOSE_FILE}" <<'EOF'
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: goolify
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-goolify}
      POSTGRES_DB: goolify
    volumes:
      - goolify-pg:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U goolify"]
      interval: 5s
      timeout: 5s
      retries: 20
    restart: unless-stopped

  # Single image: API + Vite dashboard on :8000 (host port 80)
  goolify:
    image: ghcr.io/foisalislambd/goolify:latest
    env_file: .env
    environment:
      GOOLIFY_DATABASE_URL: ${GOOLIFY_DATABASE_URL}
      GOOLIFY_HTTP_ADDR: ":8000"
    ports:
      - "80:8000"
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
  DBURL=$(grep GOOLIFY_DATABASE_URL "${ENV_FILE}" | cut -d= -f2-)
  DBPASS=$(echo "$DBURL" | sed -n 's#.*goolify:\([^@]*\)@.*#\1#p')
  if [[ -n "${DBPASS}" ]]; then
    echo "POSTGRES_PASSWORD=${DBPASS}" >> "${ENV_FILE}"
  fi
fi

echo "==> Starting Goolify..."
cd "${GOOLIFY_DIR}"
docker compose pull || true
docker compose up -d

echo ""
echo "Goolify is starting."
echo "  Dashboard: http://$(hostname -I | awk '{print $1}')/"
echo "  API:       http://$(hostname -I | awk '{print $1}')/api/v1/version"
echo "  Health:    http://$(hostname -I | awk '{print $1}')/health"
echo "  Data:      ${GOOLIFY_DIR}"
echo ""
echo "Create your first admin account in the UI."
echo "That first register auto-adds this VPS as a server (SSH + public IP + Traefik)."
echo "Or call: POST /api/v1/servers/bootstrap-self"
