-- +goose Up
ALTER TABLE server_settings
    ADD COLUMN IF NOT EXISTS wildcard_domain TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS magic_domain TEXT NOT NULL DEFAULT 'sslip.io';

ALTER TABLE services
    ADD COLUMN IF NOT EXISTS fqdn TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE services DROP COLUMN IF EXISTS fqdn;
ALTER TABLE server_settings DROP COLUMN IF EXISTS magic_domain;
ALTER TABLE server_settings DROP COLUMN IF EXISTS wildcard_domain;
