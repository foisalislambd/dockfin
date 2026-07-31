#!/usr/bin/env bash
set -euo pipefail

# Goolify — one-command production install (Ubuntu/Debian)
# curl -fsSL https://raw.githubusercontent.com/foisalislambd/goolify/main/scripts/install.sh | sudo bash
#
# Fully automatic: Docker, secrets, Postgres, dashboard. No manual .env editing.

GOOLIFY_DIR="${GOOLIFY_DIR:-/data/goolify}"
COMPOSE_FILE="${GOOLIFY_DIR}/docker-compose.yml"
ENV_FILE="${GOOLIFY_DIR}/.env"
IMAGE="${GOOLIFY_IMAGE:-foisalislambd/goolify:latest}"

echo "==> Goolify installer (automatic setup)"
echo "    Install dir: ${GOOLIFY_DIR}"
echo "    Image:       ${IMAGE}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: curl -fsSL …/install.sh | sudo bash"
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "==> Installing Docker…"
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose plugin required (install Docker Desktop / docker-compose-plugin)."
  exit 1
fi

mkdir -p "${GOOLIFY_DIR}/data"
cd "${GOOLIFY_DIR}"

# Detect public IP for dashboard URL (no user input)
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
  echo "==> Generated secure secrets automatically"
else
  # Keep existing install; refresh public URL if empty
  if ! grep -q '^POSTGRES_PASSWORD=' "${ENV_FILE}"; then
    DBURL=$(grep '^GOOLIFY_DATABASE_URL=' "${ENV_FILE}" | cut -d= -f2-)
    DBPASS=$(echo "$DBURL" | sed -n 's#.*goolify:\([^@]*\)@.*#\1#p')
    if [[ -n "${DBPASS}" ]]; then
      echo "POSTGRES_PASSWORD=${DBPASS}" >> "${ENV_FILE}"
    fi
  fi
  echo "==> Using existing ${ENV_FILE}"
fi

# Always (re)write compose so image/tag stays current — secrets stay in .env
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
    env_file: .env
    environment:
      # Keep HTTP bind only; DB URL must come from env_file (not host shell).
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

echo "==> Starting Goolify…"
# Avoid host-exported GOOLIFY_* overriding compose/.env (breaks DB hostname).
unset GOOLIFY_DATABASE_URL GOOLIFY_MASTER_KEY GOOLIFY_HTTP_ADDR GOOLIFY_PUBLIC_URL 2>/dev/null || true
# Local rebuilds: never pull. Registry installs: pull then up.
if [[ "${IMAGE}" == *":local"* ]] || [[ "${GOOLIFY_SKIP_PULL:-}" == "1" ]]; then
  echo "    Using local image (skip pull): ${IMAGE}"
  docker compose up -d --pull never
else
  docker compose pull || true
  docker compose up -d
fi

echo "==> Waiting for health…"
ok=0
for i in $(seq 1 60); do
  if curl -fsS --max-time 2 "http://127.0.0.1/health" >/dev/null 2>&1 \
    || curl -fsS --max-time 2 "http://127.0.0.1:8000/health" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 2
done

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
IP="${IP:-$PUBLIC_IP}"

echo ""
if [[ "${ok}" -eq 1 ]]; then
  echo "Goolify is ready."
else
  echo "Goolify containers are starting (health check still warming up)."
  echo "Check: cd ${GOOLIFY_DIR} && docker compose logs -f"
fi
echo ""
echo "  Open:     http://${IP}/"
echo "  Health:   http://${IP}/health"
echo "  Data:     ${GOOLIFY_DIR}"
echo ""
echo "Next: open the URL → Register your admin account."
echo "First register auto-adds this VPS as a server. Then create a project and deploy."
echo ""
echo "Update later (no config needed):"
echo "  cd ${GOOLIFY_DIR} && docker compose pull && docker compose up -d"
echo ""
