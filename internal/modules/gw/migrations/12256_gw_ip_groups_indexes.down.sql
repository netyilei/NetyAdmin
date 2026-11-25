-- +goose Down
DROP INDEX IF EXISTS uk_gw_ip_groups_name;
