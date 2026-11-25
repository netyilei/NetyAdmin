-- +goose Up
--  IP 访问控制管理面 API 权限码种子（G2：功能交付必须带权限数据；
-- ADR 0035——ipac 收拢 gw 后端点随迁 /admin/v1/open/ip-*，码=admin.gw.ip-*，
-- 与 gw/api/keys 常量同源；对偶门禁 check-rbac-perms 强制声明↔种子一致）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-gw-ip-groups-list', 'GET', '/admin/v1/open/ip-groups', 'admin.gw.ip-groups.list', 'IP 策略组列表'),
    ('rbac-api-gw-ip-groups-create', 'POST', '/admin/v1/open/ip-groups', 'admin.gw.ip-groups.write', '创建 IP 策略组'),
    ('rbac-api-gw-ip-groups-update', 'PUT', '/admin/v1/open/ip-groups/:id', 'admin.gw.ip-groups.write', '更新 IP 策略组'),
    ('rbac-api-gw-ip-groups-delete', 'DELETE', '/admin/v1/open/ip-groups/:id', 'admin.gw.ip-groups.write', '删除 IP 策略组'),
    ('rbac-api-gw-ip-rules-list', 'GET', '/admin/v1/open/ip-rules', 'admin.gw.ip-rules.list', 'IP 规则列表'),
    ('rbac-api-gw-ip-rules-create', 'POST', '/admin/v1/open/ip-rules', 'admin.gw.ip-rules.write', '创建 IP 规则'),
    ('rbac-api-gw-ip-rules-delete', 'DELETE', '/admin/v1/open/ip-rules/:id', 'admin.gw.ip-rules.write', '删除 IP 规则'),
    ('rbac-api-gw-ip-reload', 'POST', '/admin/v1/open/ip-reload', 'admin.gw.ip-reload', 'IP 配置全节点重载')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-gw-ip-groups-list', 'rbac-api-gw-ip-groups-create', 'rbac-api-gw-ip-groups-update',
    'rbac-api-gw-ip-groups-delete', 'rbac-api-gw-ip-rules-list', 'rbac-api-gw-ip-rules-create',
    'rbac-api-gw-ip-rules-delete', 'rbac-api-gw-ip-reload');
