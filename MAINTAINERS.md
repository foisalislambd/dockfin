# Maintainers

This file lists people who can merge PRs and cut releases.

| Name | Role | Contact |
|------|------|---------|
| TBD  | Maintainer | GitHub @… |

## Version scheme

- Format: `X.Y.Z` (git tag `vX.Y.Z`)
- **Y and Z are 0–9 only** → `1.0.0` … `1.0.9` → `1.1.0` … `1.9.9` → `2.0.0`
- Same version for: GitHub Release, Docker Hub, GHCR, `dockfin version`

## Release process

**Push to `main`** → [release.yml](.github/workflows/release.yml):

1. Compute next `X.Y.Z`
2. Run tests / builds
3. Publish `foisalislambd/dockfin` + `ghcr.io/<owner>/dockfin` (`:X.Y.Z` and `:latest`)
4. Create git tag + GitHub Release

### Skip release

Put any of these in the **commit message** (tip commit on the push):

- `[skip release]`
- `[skip-release]`
- `[skip_release]`

Example: `git commit -m "chore: tweak docs [skip release]"`

`workflow_dispatch` (Actions → Release → Run) always runs when started manually.

### Required secrets

| Secret | Value |
|--------|--------|
| `DOCKERHUB_USERNAME` | Docker Hub username |
| `DOCKERHUB_TOKEN` | Docker Hub access token (Read & Write) |

`GITHUB_TOKEN` is automatic.

## Decision making

- Day-to-day: PR review by at least one maintainer
- Architecture changes: discuss in an issue first
- Security: follow [SECURITY.md](SECURITY.md)
