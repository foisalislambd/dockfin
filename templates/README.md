# Service templates

Compose one-click templates shipped with Dockfin live in `compose/`.

- **Count:** ~362 YAML files (Coolify-compatible catalog)
- **Override path:** set `DOCKFIN_TEMPLATES_DIR` to load an alternate directory
- **Provenance:** adapted from Coolify’s public `templates/compose` catalog; see root `NOTICE`

Dockfin resolves magic env placeholders such as `SERVICE_URL_*` and `SERVICE_PASSWORD_*` when deploying a service from a template (Coolify-compatible).
