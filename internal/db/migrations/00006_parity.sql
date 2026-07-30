-- +goose Up
ALTER TABLE backup_executions
    ADD COLUMN IF NOT EXISTS s3_uploaded BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS s3_key TEXT NOT NULL DEFAULT '';

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS git_source_id UUID REFERENCES git_sources(id) ON DELETE SET NULL;

ALTER TABLE deployments
    ADD COLUMN IF NOT EXISTS build_server_id UUID REFERENCES servers(id) ON DELETE SET NULL;

ALTER TABLE server_settings
    ADD COLUMN IF NOT EXISTS is_swarm_manager BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS git_source_setup_states (
    state       TEXT PRIMARY KEY,
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    source_id   UUID NOT NULL REFERENCES git_sources(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS git_source_setup_states;
ALTER TABLE server_settings DROP COLUMN IF EXISTS is_swarm_manager;
ALTER TABLE deployments DROP COLUMN IF EXISTS build_server_id;
ALTER TABLE applications DROP COLUMN IF EXISTS git_source_id;
ALTER TABLE backup_executions DROP COLUMN IF EXISTS s3_key, DROP COLUMN IF EXISTS s3_uploaded;
