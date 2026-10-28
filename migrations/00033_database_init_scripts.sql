-- +goose Up
ALTER TABLE databases
    ADD COLUMN IF NOT EXISTS init_scripts_enc TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE databases DROP COLUMN IF EXISTS init_scripts_enc;
