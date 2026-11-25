-- +goose Down
DROP INDEX IF EXISTS idx_rbac_role_menus_menu;
