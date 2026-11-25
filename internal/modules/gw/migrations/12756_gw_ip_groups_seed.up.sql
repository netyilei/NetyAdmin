-- +goose Up
-- gw_ip_groups seed（11254 配对；内置组=「默认黑名单组」示例——部署者可改）。
INSERT INTO gw_ip_groups (id, name, remark) VALUES
    ('gw-ip-group-default', '默认黑名单组', '内置示例组（黑名单模式——命中即拒）；可编辑或清空规则'),
    ('gw-ip-group-intranet', '内网放行组', '内网网段白名单示例（启用本组=仅内网 IP 放行）')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM gw_ip_groups WHERE id IN ('gw-ip-group-default', 'gw-ip-group-intranet');
