-- +goose Up
-- OIDC signup-when-registration-is-off must be opt-in, not on by default.
ALTER TABLE instance_settings
    ALTER COLUMN oidc_allow_register SET DEFAULT FALSE;
UPDATE instance_settings SET oidc_allow_register = FALSE
 WHERE id = 1 AND oidc_allow_register IS TRUE;

-- +goose Down
ALTER TABLE instance_settings
    ALTER COLUMN oidc_allow_register SET DEFAULT TRUE;
