-- +goose Down
DROP INDEX IF EXISTS idx_rbac_role_apis_api;
