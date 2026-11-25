-- +goose Up
-- / 管理面 API 权限码种子（G2：功能交付必须带权限数据——非超管
-- 角色可经 role_apis 授予 gw 面；act=权限码与 gw api/keys 声明同源）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-gw-apps-list', 'GET', '/admin/v1/open/apps', 'admin.gw.apps.list', '开放平台应用列表'),
    ('rbac-api-gw-apps-create', 'POST', '/admin/v1/open/apps', 'admin.gw.apps.create', '创建应用'),
    ('rbac-api-gw-apps-update', 'PUT', '/admin/v1/open/apps/:id', 'admin.gw.apps.update', '更新应用'),
    ('rbac-api-gw-apps-delete', 'DELETE', '/admin/v1/open/apps/:id', 'admin.gw.apps.delete', '删除应用'),
    ('rbac-api-gw-apps-reset', 'PUT', '/admin/v1/open/apps/:id/reset-secret', 'admin.gw.apps.reset_secret', '重置 AppSecret'),
    ('rbac-api-gw-apis-list', 'GET', '/admin/v1/open/apis', 'admin.gw.apis.list', '网关 API 列表'),
    ('rbac-api-gw-apis-create', 'POST', '/admin/v1/open/apis', 'admin.gw.apis.create', '登记网关 API'),
    ('rbac-api-gw-apis-update', 'PUT', '/admin/v1/open/apis/:id', 'admin.gw.apis.update', '更新网关 API'),
    ('rbac-api-gw-apis-delete', 'DELETE', '/admin/v1/open/apis/:id', 'admin.gw.apis.delete', '删除网关 API'),
    ('rbac-api-gw-apis-bind', 'PUT', '/admin/v1/open/apis/scope-apis', 'admin.gw.apis.bind', 'scope 白名单授权')
ON CONFLICT (id) DO NOTHING;
