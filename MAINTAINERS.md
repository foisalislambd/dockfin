# Maintainers

This file lists people who can merge PRs and cut releases.

| Name | Role | Contact |
|------|------|---------|
| TBD  | Maintainer | GitHub @… |

## Version

- Source of truth: [VERSION.md](VERSION.md) (`X.Y.Z`, git tag `vX.Y.Z`)
- Same version for: GitHub Release, Docker Hub, GHCR, `dockfin version`
- A release is created only when `VERSION.md` is **higher** than the latest `vX.Y.Z` tag
- The same version, or a lower one, does not create a release

## Release process

**Push to `main`** → [release.yml](.github/workflows/release.yml):

1. Read `X.Y.Z` from `VERSION.md`
2. Compare it with the latest `vX.Y.Z` tag
3. If it is higher: run tests / builds, publish `foisalislambd/dockfin` + `ghcr.io/<owner>/dockfin` (`:X.Y.Z` and `:latest`), then create the git tag and GitHub Release
4. If it is the same or lower: stop. No tag, no release, no image publish

To ship a release, raise the number in `VERSION.md` (for example `1.3.6` → `1.3.7`) in that commit.

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
