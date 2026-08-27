-- +goose Up
INSERT INTO oauth_settings (provider) VALUES ('oidc')
ON CONFLICT (provider) DO NOTHING;

ALTER TABLE instance_settings
    ADD COLUMN IF NOT EXISTS oidc_allow_register BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS oidc_auto_join_root BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE instance_settings
    DROP COLUMN IF EXISTS oidc_allow_register,
    DROP COLUMN IF EXISTS oidc_auto_join_root;
DELETE FROM oauth_settings WHERE provider = 'oidc';
