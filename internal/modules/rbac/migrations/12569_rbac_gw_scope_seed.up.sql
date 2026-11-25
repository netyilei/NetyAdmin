-- +goose Up
-- 接口权限页 scope 组管理面 API 权限码种子（G2：功能交付必须带权限数据；
-- 应用侧 scopes 并入 UpdateApp 后无独立授权端点，scope-apis/apis 复用
-- 12558 admin.gw.apis.* 码；对偶门禁 check-rbac-perms 强制声明↔种子一致）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-gw-scope-groups-list', 'GET', '/admin/v1/open/scope-groups', 'admin.gw.scope-groups.list', 'scope 组列表'),
    ('rbac-api-gw-scope-groups-create', 'POST', '/admin/v1/open/scope-groups', 'admin.gw.scope-groups.write', '创建 scope 组'),
    ('rbac-api-gw-scope-groups-update', 'PUT', '/admin/v1/open/scope-groups/:id', 'admin.gw.scope-groups.write', '更新 scope 组'),
    ('rbac-api-gw-scope-groups-delete', 'DELETE', '/admin/v1/open/scope-groups/:id', 'admin.gw.scope-groups.write', '删除 scope 组')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-gw-scope-groups-list', 'rbac-api-gw-scope-groups-create',
    'rbac-api-gw-scope-groups-update', 'rbac-api-gw-scope-groups-delete');
