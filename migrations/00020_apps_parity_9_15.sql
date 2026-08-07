-- +goose Up
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS redirect TEXT NOT NULL DEFAULT 'both',
    ADD COLUMN IF NOT EXISTS docker_registry_id UUID;

ALTER TABLE applications
    DROP CONSTRAINT IF EXISTS applications_redirect_check;
ALTER TABLE applications
    ADD CONSTRAINT applications_redirect_check CHECK (redirect IN ('both', 'www', 'non-www'));

ALTER TABLE application_settings
    ADD COLUMN IF NOT EXISTS is_disable_build_cache BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_git_shallow_clone_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS is_git_lfs_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_gpu_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS gpu_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS custom_docker_stop_timeout INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS custom_docker_restart_policy TEXT NOT NULL DEFAULT 'unless-stopped';

ALTER TABLE scheduled_backups
    ADD COLUMN IF NOT EXISTS volume_id UUID REFERENCES volumes(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS docker_registries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    url             TEXT NOT NULL DEFAULT 'docker.io',
    username        TEXT NOT NULL DEFAULT '',
    password_enc    TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS docker_registries_team_idx ON docker_registries(team_id);

ALTER TABLE applications
    DROP CONSTRAINT IF EXISTS applications_docker_registry_id_fkey;
ALTER TABLE applications
    ADD CONSTRAINT applications_docker_registry_id_fkey
    FOREIGN KEY (docker_registry_id) REFERENCES docker_registries(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_docker_registry_id_fkey;
ALTER TABLE applications DROP COLUMN IF EXISTS docker_registry_id;
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_redirect_check;
ALTER TABLE applications DROP COLUMN IF EXISTS redirect;

ALTER TABLE application_settings
    DROP COLUMN IF EXISTS is_disable_build_cache,
    DROP COLUMN IF EXISTS is_git_shallow_clone_enabled,
    DROP COLUMN IF EXISTS is_git_lfs_enabled,
    DROP COLUMN IF EXISTS is_gpu_enabled,
    DROP COLUMN IF EXISTS gpu_count,
    DROP COLUMN IF EXISTS custom_docker_stop_timeout,
    DROP COLUMN IF EXISTS custom_docker_restart_policy;

ALTER TABLE scheduled_backups DROP COLUMN IF EXISTS volume_id;
DROP TABLE IF EXISTS docker_registries;
