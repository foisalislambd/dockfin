# Security Policy

## Supported versions

Goolify is under active development. Security fixes are applied to the `main` branch.

| Branch | Supported |
|--------|-----------|
| `main` | Yes |
| Older tags / forks | Best effort |

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Instead, email the maintainers (or use GitHub Security Advisories when enabled for this repository) with:

- A description of the issue
- Steps to reproduce
- Impact assessment (e.g. RCE, secret leak, auth bypass)
- Any proof-of-concept (non-destructive)

We aim to acknowledge reports within **7 days** and to provide a remediation timeline after triage.

## Safe harbor

We will not pursue legal action against good-faith security research that:

- Avoids privacy violations, data destruction, and service disruption
- Does not exploit the issue beyond what is needed to demonstrate it
- Reports findings privately first

## Hardening notes for operators

- Use a strong `GOOLIFY_MASTER_KEY` (32+ random characters)
- Restrict API/dashboard exposure with a reverse proxy and TLS
- Keep SSH private keys encrypted at rest (Goolify encrypts key material in Postgres)
- Prefer host-key fingerprints after first validate (TOFU)
- Rotate webhook secrets periodically
