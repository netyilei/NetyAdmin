-- +goose Down
DROP INDEX IF EXISTS idx_gw_apis_status;
DROP INDEX IF EXISTS idx_gw_scope_groups_status;
DROP INDEX IF EXISTS idx_gw_apps_status;
