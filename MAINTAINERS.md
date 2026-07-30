# Maintainers

This file lists people who can merge PRs and cut releases.

| Name | Role | Contact |
|------|------|---------|
| TBD  | Maintainer | GitHub @… |

## Release process (draft)

1. Update [CHANGELOG.md](CHANGELOG.md) under `[Unreleased]` → version section
2. Tag `vX.Y.Z`
3. Build and publish container images (when registry is configured)
4. GitHub Release notes from CHANGELOG

## Decision making

- Day-to-day: PR review by at least one maintainer
- Architecture changes: discuss in an issue first
- Security: follow [SECURITY.md](SECURITY.md)
