-- +goose Up
-- Coolify-parity fields for Git Docker Compose applications (raw/deployable preview, per-service domains, build options).

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS docker_compose_raw TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS docker_compose TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS docker_compose_domains JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS base_directory TEXT NOT NULL DEFAULT '/',
    ADD COLUMN IF NOT EXISTS docker_compose_custom_build_command TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS docker_compose_custom_start_command TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS custom_docker_run_options TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS dockerfile_target_build TEXT NOT NULL DEFAULT '';

ALTER TABLE application_settings
    ADD COLUMN IF NOT EXISTS is_preserve_repository_enabled BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN applications.docker_compose_raw IS
    'Last loaded compose YAML from git (raw). Shown on General → Docker Compose.';
COMMENT ON COLUMN applications.docker_compose IS
    'Deployable / prepared compose YAML (Traefik labels etc). Empty when compose_prepare=false.';
COMMENT ON COLUMN applications.docker_compose_domains IS
    'Per-compose-service domain map: {"web":{"domain":"https://…"}}';
COMMENT ON COLUMN applications.base_directory IS
    'Monorepo root relative to repo (Coolify-style). Combined with docker_compose_location.';

-- +goose Down
ALTER TABLE application_settings
    DROP COLUMN IF EXISTS is_preserve_repository_enabled;

ALTER TABLE applications
    DROP COLUMN IF EXISTS dockerfile_target_build,
    DROP COLUMN IF EXISTS custom_docker_run_options,
    DROP COLUMN IF EXISTS docker_compose_custom_start_command,
    DROP COLUMN IF EXISTS docker_compose_custom_build_command,
    DROP COLUMN IF EXISTS base_directory,
    DROP COLUMN IF EXISTS docker_compose_domains,
    DROP COLUMN IF EXISTS docker_compose,
    DROP COLUMN IF EXISTS docker_compose_raw;
