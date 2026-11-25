-- +goose Up
--  前权限码对偶补登记（数-A5 清偿——check-rbac-perms 门禁首跑
-- 抓到 audit/identity/rbac 自身管理面 29 个声明未登记，精细角色授权失效）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-audit-list', 'GET', '/admin/v1/logs', 'admin.logs.list', '审计列表'),
    ('rbac-api-audit-get', 'GET', '/admin/v1/logs/:id', 'admin.logs.get', '审计详情'),
    ('rbac-api-audit-export', 'GET', '/admin/v1/logs/export', 'admin.logs.export', '审计导出'),
    ('rbac-api-id-admins-list', 'GET', '/admin/v1/admins', 'admin.admins.list', '管理员列表'),
    ('rbac-api-id-admins-get', 'GET', '/admin/v1/admins/:id', 'admin.admins.get', '管理员详情'),
    ('rbac-api-id-admins-create', 'POST', '/admin/v1/admins', 'admin.admins.create', '管理员创建'),
    ('rbac-api-id-admins-update', 'PUT', '/admin/v1/admins/:id', 'admin.admins.update', '管理员更新'),
    ('rbac-api-id-admins-delete', 'DELETE', '/admin/v1/admins/:id', 'admin.admins.delete', '管理员删除'),
    ('rbac-api-id-admins-status', 'PATCH', '/admin/v1/admins/:id/status', 'admin.admins.status', '管理员启停'),
    ('rbac-api-id-admins-reset', 'POST', '/admin/v1/admins/:id/reset-password', 'admin.admins.reset_password', '管理员重置密码'),
    ('rbac-api-id-admins-batch', 'POST', '/admin/v1/admins/batch-delete', 'admin.admins.delete', '管理员批量删除'),
    ('rbac-api-rb-roles-list', 'GET', '/admin/v1/roles', 'admin.roles.list', '角色列表'),
    ('rbac-api-rb-roles-get', 'GET', '/admin/v1/roles/:id', 'admin.roles.get', '角色详情'),
    ('rbac-api-rb-roles-create', 'POST', '/admin/v1/roles', 'admin.roles.create', '角色创建'),
    ('rbac-api-rb-roles-update', 'PUT', '/admin/v1/roles/:id', 'admin.roles.update', '角色更新'),
    ('rbac-api-rb-roles-delete', 'DELETE', '/admin/v1/roles/:id', 'admin.roles.delete', '角色删除'),
    ('rbac-api-rb-roles-bind-menus', 'PUT', '/admin/v1/roles/:id/menus', 'admin.roles.bind', '角色菜单绑定'),
    ('rbac-api-rb-roles-bind-apis', 'PUT', '/admin/v1/roles/:id/apis', 'admin.roles.bind', '角色 API 绑定'),
    ('rbac-api-rb-roles-bind-buttons', 'PUT', '/admin/v1/roles/:id/buttons', 'admin.roles.bind', '角色按钮绑定'),
    ('rbac-api-rb-menus-list', 'GET', '/admin/v1/menus', 'admin.menus.list', '菜单列表'),
    ('rbac-api-rb-menus-get', 'GET', '/admin/v1/menus/:id', 'admin.menus.get', '菜单详情'),
    ('rbac-api-rb-menus-create', 'POST', '/admin/v1/menus', 'admin.menus.create', '菜单创建'),
    ('rbac-api-rb-menus-update', 'PUT', '/admin/v1/menus/:id', 'admin.menus.update', '菜单更新'),
    ('rbac-api-rb-menus-delete', 'DELETE', '/admin/v1/menus/:id', 'admin.menus.delete', '菜单删除'),
    ('rbac-api-rb-apis-list', 'GET', '/admin/v1/apis', 'admin.apis.list', 'API 列表'),
    ('rbac-api-rb-apis-get', 'GET', '/admin/v1/apis/:id', 'admin.apis.get', 'API 详情'),
    ('rbac-api-rb-apis-create', 'POST', '/admin/v1/apis', 'admin.apis.create', 'API 创建'),
    ('rbac-api-rb-apis-update', 'PUT', '/admin/v1/apis/:id', 'admin.apis.update', 'API 更新'),
    ('rbac-api-rb-apis-delete', 'DELETE', '/admin/v1/apis/:id', 'admin.apis.delete', 'API 删除'),
    ('rbac-api-rb-buttons-list', 'GET', '/admin/v1/buttons', 'admin.buttons.list', '按钮列表'),
    ('rbac-api-rb-buttons-create', 'POST', '/admin/v1/buttons', 'admin.buttons.create', '按钮创建'),
    ('rbac-api-rb-buttons-update', 'PUT', '/admin/v1/buttons/:id', 'admin.buttons.update', '按钮更新'),
    ('rbac-api-rb-buttons-delete', 'DELETE', '/admin/v1/buttons/:id', 'admin.buttons.delete', '按钮删除')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id LIKE 'rbac-api-audit-%'
    OR id LIKE 'rbac-api-id-admins-%' OR id LIKE 'rbac-api-rb-%';
