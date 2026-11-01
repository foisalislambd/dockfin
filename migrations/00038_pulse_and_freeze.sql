-- +goose Up
-- Deploy freeze is an explicit pause for an environment. Restarts and
-- scheduled commands are unchanged; new deploys are refused until unfrozen.
ALTER TABLE environments
    ADD COLUMN IF NOT EXISTS deploy_frozen BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS deploy_freeze_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS deploy_frozen_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deploy_frozen_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS environments_team_frozen_idx
    ON environments (team_id)
    WHERE deploy_frozen;

-- Team pulse reads recent deployments and failed backups by team.
CREATE INDEX IF NOT EXISTS deployments_team_created_idx
    ON deployments (team_id, created_at DESC);

CREATE INDEX IF NOT EXISTS backup_executions_team_started_idx
    ON backup_executions (team_id, started_at DESC);

-- +goose Down
DROP INDEX IF EXISTS backup_executions_team_started_idx;
DROP INDEX IF EXISTS deployments_team_created_idx;
DROP INDEX IF EXISTS environments_team_frozen_idx;

ALTER TABLE environments
    DROP COLUMN IF EXISTS deploy_frozen_by,
    DROP COLUMN IF EXISTS deploy_frozen_at,
    DROP COLUMN IF EXISTS deploy_freeze_reason,
    DROP COLUMN IF EXISTS deploy_frozen;
