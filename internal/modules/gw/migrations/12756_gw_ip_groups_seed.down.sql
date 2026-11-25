-- +goose Down
DELETE FROM gw_ip_groups WHERE id IN ('gw-ip-group-default', 'gw-ip-group-intranet');
