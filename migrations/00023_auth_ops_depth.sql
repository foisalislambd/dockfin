-- +goose Up
-- Auth depth: OAuth account links, TOTP, password reset; auto-update status.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS totp_secret_enc TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS totp_recovery_codes_enc TEXT NOT NULL DEFAULT '';

-- Empty password_hash is allowed for OAuth-only users (application enforces).
-- Column remains NOT NULL; OAuth creates use ''.

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    provider_user_id  TEXT NOT NULL,
    email             TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS oauth_accounts_user_idx ON oauth_accounts(user_id);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS password_reset_tokens_email_idx ON password_reset_tokens(LOWER(email));

ALTER TABLE instance_settings
    ADD COLUMN IF NOT EXISTS auto_update_last_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS auto_update_last_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS auto_update_last_message TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS auth_challenges (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL DEFAULT 'totp',
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS auth_challenges;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS oauth_accounts;

ALTER TABLE instance_settings
    DROP COLUMN IF EXISTS auto_update_last_at,
    DROP COLUMN IF EXISTS auto_update_last_status,
    DROP COLUMN IF EXISTS auto_update_last_message;

ALTER TABLE users
    DROP COLUMN IF EXISTS totp_secret_enc,
    DROP COLUMN IF EXISTS totp_enabled,
    DROP COLUMN IF EXISTS totp_recovery_codes_enc;
