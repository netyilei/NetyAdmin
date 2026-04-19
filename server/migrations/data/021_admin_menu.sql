BEGIN;

    -- 核心菜单初始化
INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
VALUES 
(0, '首页', 'home', '/home', 'layout.base$view.home', 'mdi:monitor-dashboard', 0, false, '1', '2', 'route.home', NOW(), NOW()), 
(0, '管理员', 'manage', '/manage', 'layout', 'ic:outline-settings', 1, false, '1', '1', 'route.manage', NOW(), NOW()), 
(0, '用户', 'user', '/user', 'layout', 'ic:outline-people', 2, false, '1', '1', 'route.user', NOW(), NOW()),
(0, '消息中心', 'message', '/message', 'layout', 'ic:baseline-message', 3, false, '1', '1', 'route.message', NOW(), NOW()),
(0, '运维', 'ops', '/ops', 'layout', 'ic:outline-build', 4, false, '1', '1', 'route.ops', NOW(), NOW()), 
(0, '系统配置', 'settings', '/settings', 'layout', 'ic:outline-settings', 5, false, '1', '1', 'route.settings', NOW(), NOW()), 
(0, '内容管理', 'content', '/content', 'layout', 'ic:outline-article', 6, false, '1', '1', 'route.content', NOW(), NOW()),
(0, '开放平台', 'open-platform', '/open-platform', 'layout', 'ic:outline-settings-input-component', 7, false, '1', '1', 'route.open_platform', NOW(), NOW()) 
ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET 
    name = EXCLUDED.name,
    i18_n_key = EXCLUDED.i18_n_key, 
    component = EXCLUDED.component, 
    parent_id = EXCLUDED.parent_id,
    order_by = EXCLUDED.order_by; 

-- 模块子菜单初始化 
DO $$ 
DECLARE 
    manage_menu_id BIGINT; 
    user_root_id BIGINT;
    message_root_id BIGINT;
    ops_menu_id BIGINT; 
    settings_menu_id BIGINT; 
    content_root_id BIGINT;
    open_menu_id BIGINT;
BEGIN 
    SELECT id INTO manage_menu_id FROM admin_menu WHERE route_name = 'manage' AND deleted_at = 0; 
    SELECT id INTO user_root_id FROM admin_menu WHERE route_name = 'user' AND deleted_at = 0;
    SELECT id INTO message_root_id FROM admin_menu WHERE route_name = 'message' AND deleted_at = 0;
    SELECT id INTO ops_menu_id FROM admin_menu WHERE route_name = 'ops' AND deleted_at = 0; 
    SELECT id INTO settings_menu_id FROM admin_menu WHERE route_name = 'settings' AND deleted_at = 0; 
    SELECT id INTO content_root_id FROM admin_menu WHERE route_name = 'content' AND deleted_at = 0;
    SELECT id INTO open_menu_id FROM admin_menu WHERE route_name = 'open-platform' AND deleted_at = 0;
 
    -- 管理员管理子菜单 
    IF manage_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (manage_menu_id, '管理员管理', 'manage_admin', '/manage/admin', 'view.manage_admin', 'ic:round-manage-accounts', 1, false, '1', '2', 'route.manage_admin', NOW(), NOW()), 
        (manage_menu_id, '角色管理', 'manage_role', '/manage/role', 'view.manage_role', 'carbon:user-role', 2, false, '1', '2', 'route.manage_role', NOW(), NOW()), 
        (manage_menu_id, '菜单管理', 'manage_menu', '/manage/menu', 'view.manage_menu', 'material-symbols:route', 3, false, '1', '2', 'route.manage_menu', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key, component = EXCLUDED.component, parent_id = EXCLUDED.parent_id, route_path = EXCLUDED.route_path; 
    END IF; 

    -- 用户管理子菜单
    IF user_root_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (user_root_id, '用户管理', 'manage_user', '/user/manage', 'view.manage_user', 'ic:round-people', 1, false, '1', '2', 'route.manage_user', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key, component = EXCLUDED.component, parent_id = EXCLUDED.parent_id, route_path = EXCLUDED.route_path;
    END IF;

    -- 消息中心子菜单
    IF message_root_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (message_root_id, '发送短信', 'message_send-sms', '/message/send-sms', 'view.message_send-sms', 'ic:baseline-sms', 1, false, '1', '2', 'route.message_send_sms', NOW(), NOW()),
        (message_root_id, '发送邮件', 'message_send-email', '/message/send-email', 'view.message_send-email', 'ic:baseline-email', 2, false, '1', '2', 'route.message_send_email', NOW(), NOW()),
        (message_root_id, '发送站内信', 'message_send-internal', '/message/send-internal', 'view.message_send-internal', 'ic:baseline-notifications', 3, false, '1', '2', 'route.message_send_internal', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key, component = EXCLUDED.component, parent_id = EXCLUDED.parent_id, route_path = EXCLUDED.route_path;
    END IF;
 
    -- 运维管理子菜单 
    IF ops_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (ops_menu_id, '操作日志', 'ops_operation-log', '/ops/operation-log', 'view.ops_operation-log', 'ic:outline-history', 1, false, '1', '2', 'route.ops_operation-log', NOW(), NOW()), 
        (ops_menu_id, '错误日志', 'ops_error-log', '/ops/error-log', 'view.ops_error-log', 'ic:outline-error-outline', 2, false, '1', '2', 'route.ops_error-log', NOW(), NOW()), 
        (ops_menu_id, '任务调度', 'ops_task', '/ops/task', 'view.ops_task', 'ic:outline-schedule', 3, false, '1', '2', 'route.ops_task', NOW(), NOW()),
        (ops_menu_id, 'IP 访问控制', 'ops_ip-access', '/ops/ip-access', 'view.ops_ip-access', 'ic:baseline-security', 4, false, '1', '2', 'route.ops_ip-access', NOW(), NOW()),
        (ops_menu_id, '上传记录', 'ops_upload-record', '/ops/upload-record', 'view.ops_upload-record', 'ic:outline-cloud-upload', 5, false, '1', '2', 'route.ops_upload-record', NOW(), NOW()),
        (ops_menu_id, '调用日志', 'ops_open-platform-log', '/ops/open-platform-log', 'view.ops_open-platform-log', 'ic:outline-history', 6, false, '1', '2', 'route.ops_open-platform-log', NOW(), NOW()),
        (ops_menu_id, '消息记录', 'ops_message-log', '/ops/message-log', 'view.ops_message-log', 'ic:baseline-history', 7, false, '1', '2', 'route.ops_message-log', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key, component = EXCLUDED.component, parent_id = EXCLUDED.parent_id, route_path = EXCLUDED.route_path; 
    END IF; 
 
    -- 基础设置子菜单 
    IF settings_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (settings_menu_id, '系统设置', 'manage_system_setting', '/settings/system', 'view.manage_system_setting', 'carbon:settings-adjust', 1, false, '1', '2', 'route.manage_system_setting', NOW(), NOW()), 
        (settings_menu_id, '字典管理', 'manage_dict', '/settings/dict', 'view.manage_dict', 'mdi:book-open-variant', 2, false, '1', '2', 'route.manage_dict', NOW(), NOW()),
        (settings_menu_id, '存储配置', 'settings_storage-config', '/settings/storage-config', 'view.settings_storage-config', 'ic:outline-cloud-queue', 3, false, '1', '2', 'route.settings_storage-config', NOW(), NOW()),
        (settings_menu_id, '消息模板', 'settings_message-template', '/settings/message-template', 'view.settings_message-template', 'mdi:file-document-edit-outline', 4, false, '1', '2', 'route.settings_message-template', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key, component = EXCLUDED.component, parent_id = EXCLUDED.parent_id, route_path = EXCLUDED.route_path; 
    END IF; 

    -- 内容管理子菜单
    IF content_root_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        VALUES 
        (content_root_id, '内容分类', 'content_category', '/content/category', 'view.content_category', 'ic:outline-category', 1, false, '1', '2', 'route.content_category', NOW(), NOW()),
        (content_root_id, '文章管理', 'content_article', '/content/article', 'view.content_article', 'ic:outline-article', 2, false, '1', '2', 'route.content_article', NOW(), NOW()),
        (content_root_id, 'Banner组', 'content_banner-group', '/content/banner-group', 'view.content_banner-group', 'ic:outline-collections', 3, false, '1', '2', 'route.content_banner-group', NOW(), NOW()),
        (content_root_id, 'Banner管理', 'content_banner', '/content/banner', 'view.content_banner', 'ic:outline-image', 4, true, '1', '2', 'route.content_banner', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key, component = EXCLUDED.component, parent_id = EXCLUDED.parent_id;
    END IF;

    -- 开放平台子菜单
    IF open_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (open_menu_id, '应用管理', 'open-platform_apps', '/open-platform/apps', 'view.open-platform_apps', 'ic:baseline-apps', 1, false, '1', '2', 'route.open-platform_apps', NOW(), NOW()),
        (open_menu_id, 'API管理', 'open-platform_apis', '/open-platform/apis', 'view.open-platform_apis', 'ic:baseline-api', 2, false, '1', '2', 'route.open-platform_apis', NOW(), NOW()),
        (open_menu_id, '接口权限', 'open-platform_scopes', '/open-platform/scopes', 'view.open-platform_scopes', 'ic:baseline-security', 3, false, '1', '2', 'route.open-platform_scopes', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key, component = EXCLUDED.component, parent_id = EXCLUDED.parent_id, route_path = EXCLUDED.route_path; 
    END IF; 
 END $$;

COMMIT;
