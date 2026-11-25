-- +goose Down
DROP INDEX IF EXISTS idx_rbac_admin_roles_role;
