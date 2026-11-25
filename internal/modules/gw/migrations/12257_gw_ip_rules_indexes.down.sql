-- +goose Down
DROP INDEX IF EXISTS idx_gw_ip_rules_group;
