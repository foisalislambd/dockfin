# VPS one-click smoke test

Install Goolify and run API + deploy smoke tests on a fresh Ubuntu/Debian VPS with a single command.

## What it does

1. Installs Docker, Go, jq, and OpenSSH  
2. Starts Postgres/Redis (`deploy/compose/docker-compose.dev.yml`)  
3. Builds the `goolify` binary, migrates the DB, and starts the API  
4. Registers a test user  
5. Sets up self-SSH (VPS targeting itself): key → server → validate  
6. Starts the Traefik proxy  
7. Creates a project and deploys `nginx:alpine`  
8. Polls until the deployment status is `finished`  
9. Writes a pass/fail report to `/opt/goolify-smoke/report.txt`

## Usage

### A) Repo already on the VPS

```bash
# After upload or git clone:
cd /path/to/goolify
sudo bash scripts/vps-oneclick-test.sh
```

### B) Copy the repo from another machine

```bash
# From your PC:
scp -r ./goolify root@VPS_IP:/opt/goolify

# On the VPS:
ssh root@VPS_IP
cd /opt/goolify
sudo bash scripts/vps-oneclick-test.sh
```

### C) Clone from Git

```bash
git clone https://github.com/YOUR_USER/goolify.git /opt/goolify
cd /opt/goolify
sudo bash scripts/vps-oneclick-test.sh
```

Or set a clone URL if the source is missing:

```bash
sudo GOOLIFY_GIT_URL=https://github.com/YOUR_USER/goolify.git \
  bash scripts/vps-oneclick-test.sh
```

## Options

| Env | Meaning |
|-----|---------|
| `SKIP_DEPLOY=1` | API register/health only (skip SSH deploy) |
| `SKIP_WEB=1` | Skip Node install / Vite UI build |
| `KEEP_RUNNING=0` | Stop the API after tests |
| `API_PORT=8080` | API listen port |
| `GOOLIFY_SRC=/path` | Force source path |

Example:

```bash
sudo SKIP_DEPLOY=1 bash scripts/vps-oneclick-test.sh
```

## Output

- Report: `/opt/goolify-smoke/report.txt`
- API log: `/opt/goolify-smoke/api.log`
- Test user/password printed at the end of the script
- Public VPS IP + URLs are printed, for example:
  - `API (public): http://YOUR_VPS_IP:8080`
  - `Health: http://YOUR_VPS_IP:8080/health`

## Notes

- Root is required (Docker + SSH + Traefik on port 80)
- Control plane and deploy target are the same VPS (self-SSH)
- No public GHCR image required — builds from source
- `http://VPS_IP:8080/` serves the API; after the smoke script builds `apps/web/dist`, the same URL also serves the dashboard UI
- The smoke script installs Node.js 22 and runs `npm ci && npm run build` unless `SKIP_WEB=1`
