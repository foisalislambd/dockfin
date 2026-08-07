-- +goose Up
UPDATE applications SET build_pack = 'railpack' WHERE build_pack = 'nixpacks';

ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_build_pack_check;
ALTER TABLE applications ADD CONSTRAINT applications_build_pack_check
    CHECK (build_pack IN ('dockerfile', 'dockercompose', 'dockerimage', 'static', 'railpack'));

-- +goose Down
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_build_pack_check;
ALTER TABLE applications ADD CONSTRAINT applications_build_pack_check
    CHECK (build_pack IN ('dockerfile', 'dockercompose', 'dockerimage', 'nixpacks', 'static', 'railpack'));
