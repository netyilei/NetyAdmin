-- +goose Up
-- rbac_menus seed（11051 配对—— Geeker 动态路由菜单；2026-11-21 用户
-- 层级重构：用户管理/权限管理独立一级+运维一级+网关管理一级（IP 访问控制+
-- 应用平台——网关治理更名应用平台）。
-- component=前端视图路径 /src/views<component>.vue；icon=Element Plus 图标名。
INSERT INTO rbac_menus (id, parent_id, name, path, component, icon, sort_order, hidden, status) VALUES
    ('rbac-menu-home', '', '首页', '/home/index', '/home/index', 'HomeFilled', 0, false, 1),
    ('rbac-menu-user', '', '用户管理', '/user', '', 'User', 5, false, 1),
    ('rbac-menu-admins', 'rbac-menu-user', '管理员', '/user/admins', '/system/admins/index', 'Postcard', 11, false, 1),
    ('rbac-menu-users', 'rbac-menu-user', 'C 端用户', '/user/users', '/system/users/index', 'Avatar', 12, false, 1),
    ('rbac-menu-perm', '', '权限管理', '/perm', '', 'Lock', 10, false, 1),
    ('rbac-menu-roles', 'rbac-menu-perm', '角色权限', '/perm/roles', '/system/roles/index', 'Key', 11, false, 1),
    ('rbac-menu-menus', 'rbac-menu-perm', '菜单管理', '/perm/menus', '/system/menus/index', 'Menu', 12, false, 1),
    ('rbac-menu-apis', 'rbac-menu-perm', 'API 权限', '/perm/apis', '/system/apis/index', 'List', 13, false, 1),
    ('rbac-menu-buttons', 'rbac-menu-perm', '按钮权限', '/perm/buttons', '/system/buttons/index', 'Pointer', 14, false, 1),
    ('rbac-menu-system', '', '系统管理', '/system', '', 'Setting', 20, false, 1),
    ('rbac-menu-configs', 'rbac-menu-system', '系统配置', '/system/configs', '/system/configs/index', 'Tools', 21, false, 1),
    ('rbac-menu-ops', '', '运维', '/ops', '', 'Monitor', 30, false, 1),
    ('rbac-menu-logs', 'rbac-menu-ops', '操作日志', '/ops/logs', '/system/logs/index', 'Document', 31, false, 1),
    ('rbac-menu-taskcenter', 'rbac-menu-ops', '任务管理', '/ops/taskcenter', '/system/taskcenter/index', 'Timer', 32, false, 1),
    ('rbac-menu-message', 'rbac-menu-ops', '消息管理', '/ops/message', '/system/message/index', 'Message', 33, false, 1),
    ('rbac-menu-storage', 'rbac-menu-ops', '存储配置', '/ops/storage', '/system/storage/index', 'UploadFilled', 34, false, 1),
    ('rbac-menu-gwmgmt', '', '网关管理', '/gw-mgmt', '', 'Connection', 40, false, 1),
    ('rbac-menu-gw', 'rbac-menu-gwmgmt', '应用平台', '/gw-mgmt/apps', '/system/gw/index', 'OfficeBuilding', 41, false, 1),
    ('rbac-menu-ipac', 'rbac-menu-gwmgmt', 'IP 访问控制', '/gw-mgmt/ipac', '/system/ipac/index', 'Filter', 42, false, 1),
    ('rbac-menu-gwscope', 'rbac-menu-gwmgmt', '接口权限', '/gw-mgmt/scope', '/system/gwscope/index', 'Link', 43, false, 1),
    ('rbac-menu-gwapi', 'rbac-menu-gwmgmt', 'API 登记', '/gw-mgmt/api', '/system/gwapi/index', 'Files', 44, false, 1)
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_menus WHERE id IN ('rbac-menu-home', 'rbac-menu-user', 'rbac-menu-admins', 'rbac-menu-users', 'rbac-menu-perm', 'rbac-menu-roles', 'rbac-menu-menus', 'rbac-menu-apis', 'rbac-menu-buttons', 'rbac-menu-system', 'rbac-menu-configs', 'rbac-menu-ops', 'rbac-menu-logs', 'rbac-menu-taskcenter', 'rbac-menu-message', 'rbac-menu-storage', 'rbac-menu-gwmgmt', 'rbac-menu-ipac', 'rbac-menu-gwapi', 'rbac-menu-gwscope', 'rbac-menu-gw');
