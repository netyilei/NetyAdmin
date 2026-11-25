-- +goose Up
CREATE UNIQUE INDEX uk_gw_ip_groups_name ON gw_ip_groups (name);
-- +goose Down
DROP INDEX IF EXISTS uk_gw_ip_groups_name;
