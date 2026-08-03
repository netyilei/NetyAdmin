-- 0043_seed_admin_menu.down.sql
-- 回滚 0043：删除种子初始化的菜单（按 route_name 精确匹配）
-- 删除顺序：先删子菜单（parent_id 非零），再删根菜单
DELETE FROM admin_menu
WHERE route_name IN (
    -- 二级菜单（子菜单）
    'manage_admin', 'manage_role', 'manage_menu',
    'manage_user',
    'message_send-sms', 'message_send-email', 'message_send-internal',
    'ops_operation-log', 'ops_error-log', 'ops_task', 'ops_upload-record', 'ops_open-platform-log', 'ops_message-log',
    'manage_system_setting', 'manage_dict', 'settings_storage-config', 'settings_message-template',
    'content_category', 'content_article', 'content_banner-group', 'content_banner',
    'open-platform_apps', 'open-platform_apis', 'open-platform_scopes', 'open-platform_ip-access'
);

DELETE FROM admin_menu
WHERE route_name IN (
    -- 一级菜单（根菜单）
    'home', 'manage', 'user', 'message', 'ops', 'settings', 'content', 'open-platform'
);
