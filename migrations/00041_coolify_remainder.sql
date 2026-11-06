-- +goose Up
ALTER TABLE team_members
    ADD COLUMN IF NOT EXISTS access_scope TEXT NOT NULL DEFAULT 'all';

ALTER TABLE team_members
    DROP CONSTRAINT IF EXISTS team_members_access_scope_check;
ALTER TABLE team_members
    ADD CONSTRAINT team_members_access_scope_check CHECK (access_scope IN ('all', 'projects'));

CREATE TABLE IF NOT EXISTS project_acl (
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    permission  TEXT NOT NULL DEFAULT 'view' CHECK (permission IN ('view', 'deploy', 'manage')),
    PRIMARY KEY (team_id, user_id, project_id)
);

CREATE TABLE IF NOT EXISTS passkeys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL DEFAULT 'Passkey',
    credential_id   BYTEA NOT NULL UNIQUE,
    credential      JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS passkeys_user_idx ON passkeys(user_id);

CREATE TABLE IF NOT EXISTS webauthn_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('register', 'login')),
    session     JSONB NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS passkey_settings (
    id      INT PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);
INSERT INTO passkey_settings (id, enabled) VALUES (1, TRUE) ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS gitlab_system_hooks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    gitlab_url  TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    token_enc   TEXT NOT NULL DEFAULT '',
    hook_id     BIGINT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE servers
    ADD COLUMN IF NOT EXISTS hostinger_vm_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE servers DROP COLUMN IF EXISTS hostinger_vm_id;
DROP TABLE IF EXISTS gitlab_system_hooks;
DROP TABLE IF EXISTS passkey_settings;
DROP TABLE IF EXISTS webauthn_sessions;
DROP TABLE IF EXISTS passkeys;
DROP TABLE IF EXISTS project_acl;
ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_access_scope_check;
ALTER TABLE team_members DROP COLUMN IF EXISTS access_scope;
