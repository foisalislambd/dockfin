-- +goose Up
CREATE TABLE IF NOT EXISTS background_jobs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id       UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL,
    payload       JSONB NOT NULL DEFAULT '{}'::jsonb,
    status        TEXT NOT NULL DEFAULT 'queued'
                  CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    attempts      INTEGER NOT NULL DEFAULT 0,
    max_attempts  INTEGER NOT NULL DEFAULT 3,
    run_after     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_until  TIMESTAMPTZ,
    error         TEXT NOT NULL DEFAULT '',
    result        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS background_jobs_ready_idx
    ON background_jobs (status, run_after, created_at);

-- +goose Down
DROP TABLE IF EXISTS background_jobs;
