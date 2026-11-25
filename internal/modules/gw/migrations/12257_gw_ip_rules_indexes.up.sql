-- +goose Up
CREATE INDEX idx_gw_ip_rules_group ON gw_ip_rules (group_id);
-- +goose Down
DROP INDEX IF EXISTS idx_gw_ip_rules_group;
