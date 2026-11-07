-- +goose Up
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS delete_error TEXT NOT NULL DEFAULT '';

ALTER TABLE databases
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS delete_error TEXT NOT NULL DEFAULT '';

ALTER TABLE services
    ADD COLUMN IF NOT EXISTS delete_error TEXT NOT NULL DEFAULT '';

ALTER TABLE application_previews
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS delete_error TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE application_previews DROP COLUMN IF EXISTS delete_error, DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE services DROP COLUMN IF EXISTS delete_error;
ALTER TABLE databases DROP COLUMN IF EXISTS delete_error, DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE applications DROP COLUMN IF EXISTS delete_error, DROP COLUMN IF EXISTS deleted_at;
