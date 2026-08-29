-- +goose Up
ALTER TABLE servers
    ADD COLUMN IF NOT EXISTS jump_host_id UUID REFERENCES servers(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS servers_jump_host_id_idx ON servers(jump_host_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id        UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    method         TEXT NOT NULL DEFAULT '',
    path           TEXT NOT NULL DEFAULT '',
    action         TEXT NOT NULL DEFAULT '',
    resource_type  TEXT NOT NULL DEFAULT '',
    resource_id    TEXT NOT NULL DEFAULT '',
    status_code    INTEGER NOT NULL DEFAULT 0,
    ip             TEXT NOT NULL DEFAULT '',
    user_agent     TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS audit_logs_team_created_idx ON audit_logs(team_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP INDEX IF EXISTS servers_jump_host_id_idx;
ALTER TABLE servers DROP COLUMN IF EXISTS jump_host_id;
