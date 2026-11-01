-- +goose Up
-- Audit rows gain a short summary and a category so the log can be filtered
-- without parsing paths. team_id becomes nullable so an auth attempt that
-- does not match a user can still be stored.
ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS actor_email TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'mutation';

ALTER TABLE audit_logs ALTER COLUMN team_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS audit_logs_team_category_created_idx
    ON audit_logs (team_id, category, created_at DESC);

CREATE INDEX IF NOT EXISTS audit_logs_created_idx ON audit_logs (created_at);

-- Shared sliding-window hits for API replicas. The in-process limiter stays
-- the default; this table is used when DOCKFIN_RATE_LIMIT_STORE=postgres.
CREATE TABLE IF NOT EXISTS rate_limit_hits (
    bucket_key TEXT NOT NULL,
    seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS rate_limit_hits_key_seen_idx
    ON rate_limit_hits (bucket_key, seen_at);

-- +goose Down
DROP TABLE IF EXISTS rate_limit_hits;

DROP INDEX IF EXISTS audit_logs_created_idx;
DROP INDEX IF EXISTS audit_logs_team_category_created_idx;

DELETE FROM audit_logs WHERE team_id IS NULL;

ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS category,
    DROP COLUMN IF EXISTS actor_email,
    DROP COLUMN IF EXISTS summary;

ALTER TABLE audit_logs ALTER COLUMN team_id SET NOT NULL;
