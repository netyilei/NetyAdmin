-- +goose Down
DROP INDEX IF EXISTS idx_rbac_apis_status;
