-- +goose Up
ALTER TABLE services
    ADD COLUMN IF NOT EXISTS is_force_https BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down
ALTER TABLE services DROP COLUMN IF EXISTS is_force_https;
