-- +goose Down
DROP INDEX IF EXISTS idx_rbac_roles_status;
