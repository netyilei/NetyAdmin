-- +goose Down
DELETE FROM gw_ip_rules WHERE id = 'gw-ip-rule-example';
