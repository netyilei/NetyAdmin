-- +goose Down
ALTER TABLE rbac_menus DROP COLUMN IF EXISTS module;
