-- +goose Up
CREATE INDEX idx_gw_apps_status ON gw_apps (status);
CREATE INDEX idx_gw_scope_groups_status ON gw_scope_groups (status);
CREATE INDEX idx_gw_apis_status ON gw_apis (status);
-- +goose Down
DROP INDEX IF EXISTS idx_gw_apis_status;
DROP INDEX IF EXISTS idx_gw_scope_groups_status;
DROP INDEX IF EXISTS idx_gw_apps_status;
