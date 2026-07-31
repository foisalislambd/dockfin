# Maintainers

This file lists people who can merge PRs and cut releases.

| Name | Role | Contact |
|------|------|---------|
| TBD  | Maintainer | GitHub @… |

## Version scheme

- Format: `X.Y.Z` (git tag `vX.Y.Z`)
- **Y and Z are 0–9 only** → `1.0.0` … `1.0.9` → `1.1.0` … `1.9.9` → `2.0.0`
- Same version for: GitHub Release, Docker tags, `goolify version`, `/api/v1/version`

## Release process

**Push to `main`** → [release.yml](.github/workflows/release.yml):

1. Compute next `X.Y.Z` from latest `v*` tag (first = `1.0.0`)
2. Run tests / builds
3. Publish Docker `goolify:X.Y.Z` + `goolify-web:X.Y.Z` (+ `:latest`)
4. Create git tag `vX.Y.Z` + GitHub Release (same version)

Skip: put `[skip release]` in the commit message.

## Decision making

- Day-to-day: PR review by at least one maintainer
- Architecture changes: discuss in an issue first
- Security: follow [SECURITY.md](SECURITY.md)
