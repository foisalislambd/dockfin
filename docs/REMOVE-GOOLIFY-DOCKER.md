# Remove old Goolify Docker images / packages

After renaming to **Dockfin**, clean leftover **goolify** images and registry packages so nothing old is pulled by mistake.

## 1. Local Docker (this VPS / laptop)

```bash
# List anything still named goolify
docker images | grep -i goolify || true
docker ps -a --filter name=goolify || true

# Remove containers (if any)
docker ps -aq --filter name=goolify | xargs -r docker rm -f

# Remove images
docker images -q --filter reference='*goolify*' | xargs -r docker rmi -f
docker images -q --filter reference='goolify:*' | xargs -r docker rmi -f
docker images -q --filter reference='ghcr.io/*/goolify*' | xargs -r docker rmi -f
docker images -q --filter reference='*/goolify*' | xargs -r docker rmi -f

# Optional: prune dangling layers
docker image prune -f
```

Verify:

```bash
docker images | grep -iE 'goolify|dockfin' || echo 'no matching images'
```

## 2. GitHub Container Registry (GHCR) — delete the package

GHCR package name follows the **repo / package** name. Old package:

`ghcr.io/foisalislambd/goolify`

### Via GitHub website

1. Open: [https://github.com/users/foisalislambd/packages](https://github.com/users/foisalislambd/packages)  
   (or org packages if it lives under an org)
2. Click the **goolify** package
3. **Package settings** (right sidebar) → **Delete this package**
4. Type the package name to confirm

After you rename the GitHub repo to `dockfin`, new publishes go to:

`ghcr.io/foisalislambd/dockfin`

### Via GitHub CLI (`gh`)

```bash
# List packages (user scope)
gh api user/packages?package_type=container --jq '.[].name'

# Delete the whole goolify container package (destructive)
gh api -X DELETE \
  "/user/packages/container/goolify"

# If GitHub asks for confirm via version delete first, list versions then delete:
gh api "/user/packages/container/goolify/versions" --jq '.[].id'
# gh api -X DELETE "/user/packages/container/goolify/versions/<ID>"
```

Org-owned package (replace `ORG`):

```bash
gh api -X DELETE "/orgs/ORG/packages/container/goolify"
```

## 3. Docker Hub (if you mirrored)

```bash
# Browser: https://hub.docker.com/repository/docker/foisalislambd/goolify/general
# → Settings → Delete repository

# Or Hub API / UI only — docker CLI cannot delete Hub repos.
```

## 4. Host leftovers (optional)

```bash
# Old install dir (already wiped if you ran the rename cleanup)
sudo rm -rf /data/goolify

# Old smoke binary tree
sudo rm -rf /opt/goolify-smoke
```

## 5. After cleanup — install Dockfin

From the repo (local image):

```bash
cd /path/to/dockfin   # or /root/goolify until you rename the folder
sudo bash scripts/install-dev.sh
```

Production (after GHCR has `dockfin` images):

```bash
curl -fsSL https://raw.githubusercontent.com/foisalislambd/dockfin/main/scripts/install.sh | sudo bash
```

---

**Note:** Deleting GHCR/Hub packages is permanent. Local `docker rmi` only affects this machine.
