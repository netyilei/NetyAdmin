-- +goose Down
DROP INDEX IF EXISTS idx_rbac_menus_parent;
