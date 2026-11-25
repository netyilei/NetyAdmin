-- +goose Up
-- gw_ip_rules seed（11255 配对——示例规则挂默认黑名单组；部署者按需增删）。
INSERT INTO gw_ip_rules (id, group_id, cidrs, action, priority, enabled, remark) VALUES
    ('gw-ip-rule-example', 'gw-ip-group-default', '203.0.113.0/24', 2, 0, false, '示例：黑名单网段（默认停用——启用前改为你真实要封的段）')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM gw_ip_rules WHERE id = 'gw-ip-rule-example';
