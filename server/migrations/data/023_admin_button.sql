BEGIN;

-- 系统核心按钮初始化
DO $$
DECLARE
    admin_menu_id BIGINT;
    role_menu_id BIGINT;
    menu_menu_id BIGINT;
    button_menu_id BIGINT;
    api_menu_id BIGINT;
    user_menu_id BIGINT;
    dict_menu_id BIGINT;
    ipac_menu_id BIGINT;
    category_menu_id BIGINT;
    article_menu_id BIGINT;
    banner_group_menu_id BIGINT;
    banner_item_menu_id BIGINT;
    storage_config_menu_id BIGINT;
    upload_record_menu_id BIGINT;
    app_menu_id BIGINT;
    open_api_menu_id BIGINT;
    scope_menu_id BIGINT;
    open_log_menu_id BIGINT;
    msg_tpl_menu_id BIGINT;
    msg_sms_menu_id BIGINT;
    msg_email_menu_id BIGINT;
    msg_internal_menu_id BIGINT;
    msg_log_menu_id BIGINT;
BEGIN
    SELECT id INTO admin_menu_id FROM admin_menu WHERE route_name = 'manage_admin' AND deleted_at = 0;
    SELECT id INTO user_menu_id FROM admin_menu WHERE route_name = 'manage_user' AND deleted_at = 0;
    SELECT id INTO role_menu_id FROM admin_menu WHERE route_name = 'manage_role' AND deleted_at = 0;
    SELECT id INTO menu_menu_id FROM admin_menu WHERE route_name = 'manage_menu' AND deleted_at = 0;
    SELECT id INTO button_menu_id FROM admin_menu WHERE route_name = 'manage_button' AND deleted_at = 0;
    SELECT id INTO api_menu_id FROM admin_menu WHERE route_name = 'manage_api' AND deleted_at = 0;
    SELECT id INTO dict_menu_id FROM admin_menu WHERE route_name = 'manage_dict' AND deleted_at = 0;
    SELECT id INTO ipac_menu_id FROM admin_menu WHERE route_name = 'ops_ip-access' AND deleted_at = 0;
    SELECT id INTO category_menu_id FROM admin_menu WHERE route_name = 'content_category' AND deleted_at = 0;
    SELECT id INTO article_menu_id FROM admin_menu WHERE route_name = 'content_article' AND deleted_at = 0;
    SELECT id INTO banner_group_menu_id FROM admin_menu WHERE route_name = 'content_banner-group' AND deleted_at = 0;
    SELECT id INTO banner_item_menu_id FROM admin_menu WHERE route_name = 'content_banner' AND deleted_at = 0;
    SELECT id INTO storage_config_menu_id FROM admin_menu WHERE route_name = 'settings_storage-config' AND deleted_at = 0;
    SELECT id INTO upload_record_menu_id FROM admin_menu WHERE route_name = 'ops_upload-record' AND deleted_at = 0;
    SELECT id INTO app_menu_id FROM admin_menu WHERE route_name = 'open-platform_apps' AND deleted_at = 0;
    SELECT id INTO open_api_menu_id FROM admin_menu WHERE route_name = 'open-platform_apis' AND deleted_at = 0;
    SELECT id INTO scope_menu_id FROM admin_menu WHERE route_name = 'open-platform_scopes' AND deleted_at = 0;
    SELECT id INTO open_log_menu_id FROM admin_menu WHERE route_name = 'ops_open-platform-log' AND deleted_at = 0;
    SELECT id INTO msg_tpl_menu_id FROM admin_menu WHERE route_name = 'settings_message-template' AND deleted_at = 0;
    SELECT id INTO msg_sms_menu_id FROM admin_menu WHERE route_name = 'message_send-sms' AND deleted_at = 0;
    SELECT id INTO msg_email_menu_id FROM admin_menu WHERE route_name = 'message_send-email' AND deleted_at = 0;
    SELECT id INTO msg_internal_menu_id FROM admin_menu WHERE route_name = 'message_send-internal' AND deleted_at = 0;
    SELECT id INTO msg_log_menu_id FROM admin_menu WHERE route_name = 'ops_message-log' AND deleted_at = 0;

    -- 管理员管理按钮
    IF admin_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (admin_menu_id, 'user:add', '新增', NOW(), NOW()),
        (admin_menu_id, 'user:edit', '编辑', NOW(), NOW()),
        (admin_menu_id, 'user:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 终端用户管理按钮
    IF user_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (user_menu_id, '查询', 'user:query', NOW(), NOW()),
        (user_menu_id, '新增', 'user:add', NOW(), NOW()),
        (user_menu_id, '编辑', 'user:edit', NOW(), NOW()),
        (user_menu_id, '删除', 'user:delete', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 角色管理按钮
    IF role_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (role_menu_id, 'role:add', '新增', NOW(), NOW()),
        (role_menu_id, 'role:edit', '编辑', NOW(), NOW()),
        (role_menu_id, 'role:delete', '删除', NOW(), NOW()),
        (role_menu_id, 'role:auth', '菜单权限', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 菜单管理按钮
    IF menu_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (menu_menu_id, 'menu:add', '新增', NOW(), NOW()),
        (menu_menu_id, 'menu:edit', '编辑', NOW(), NOW()),
        (menu_menu_id, 'menu:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 按钮管理按钮
    IF button_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (button_menu_id, 'button:add', '新增', NOW(), NOW()),
        (button_menu_id, 'button:edit', '编辑', NOW(), NOW()),
        (button_menu_id, 'button:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- API管理按钮
    IF api_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (api_menu_id, 'api:add', '新增', NOW(), NOW()),
        (api_menu_id, 'api:edit', '编辑', NOW(), NOW()),
        (api_menu_id, 'api:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 字典管理按钮
    IF dict_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (dict_menu_id, 'dict:add', '新增', NOW(), NOW()),
        (dict_menu_id, 'dict:edit', '编辑', NOW(), NOW()),
        (dict_menu_id, 'dict:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- IP 访问控制按钮权限
    IF ipac_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (ipac_menu_id, '查询', 'ip:access:query', NOW(), NOW()),
        (ipac_menu_id, '新增', 'ip:access:add', NOW(), NOW()),
        (ipac_menu_id, '编辑', 'ip:access:edit', NOW(), NOW()),
        (ipac_menu_id, '删除', 'ip:access:delete', NOW(), NOW()),
        (ipac_menu_id, '批量删除', 'ip:access:batchDelete', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 分类管理按钮
    IF category_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (category_menu_id, 'content:category:add', '新增分类', NOW(), NOW()),
        (category_menu_id, 'content:category:edit', '编辑分类', NOW(), NOW()),
        (category_menu_id, 'content:category:delete', '删除分类', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 文章管理按钮
    IF article_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (article_menu_id, 'content:article:add', '发布文章', NOW(), NOW()),
        (article_menu_id, 'content:article:edit', '修改文章', NOW(), NOW()),
        (article_menu_id, 'content:article:delete', '删除', NOW(), NOW()),
        (article_menu_id, 'content:article:publish', '上架', NOW(), NOW()),
        (article_menu_id, 'content:article:unpublish', '下架', NOW(), NOW()),
        (article_menu_id, 'content:article:top', '置顶', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- Banner管理按钮
    IF banner_group_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (banner_group_menu_id, 'content:banner-group:add', '新增', NOW(), NOW()),
        (banner_group_menu_id, 'content:banner-group:edit', '编辑', NOW(), NOW()),
        (banner_group_menu_id, 'content:banner-group:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF banner_item_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (banner_item_menu_id, 'content:banner:add', '新增', NOW(), NOW()),
        (banner_item_menu_id, 'content:banner:edit', '编辑', NOW(), NOW()),
        (banner_item_menu_id, 'content:banner:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 存储配置按钮
    IF storage_config_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (storage_config_menu_id, 'storage:add', '新增', NOW(), NOW()),
        (storage_config_menu_id, 'storage:edit', '编辑', NOW(), NOW()),
        (storage_config_menu_id, 'storage:delete', '删除', NOW(), NOW()),
        (storage_config_menu_id, 'storage:test', '测试', NOW(), NOW()),
        (storage_config_menu_id, 'storage:default', '设为默认', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 上传记录按钮
    IF upload_record_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (upload_record_menu_id, 'ops:upload-record:delete', '删除', NOW(), NOW()),
        (upload_record_menu_id, 'ops:upload-record:batch-delete', '批量删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 应用管理按钮权限
    IF app_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (app_menu_id, '查询', 'open:app:query', NOW(), NOW()),
        (app_menu_id, '新增', 'open:app:add', NOW(), NOW()),
        (app_menu_id, '编辑', 'open:app:edit', NOW(), NOW()),
        (app_menu_id, '重置密钥', 'open:app:resetSecret', NOW(), NOW()),
        (app_menu_id, '删除', 'open:app:delete', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- API管理按钮权限
    IF open_api_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (open_api_menu_id, '查询', 'open:api:query', NOW(), NOW()),
        (open_api_menu_id, '新增', 'open:api:add', NOW(), NOW()),
        (open_api_menu_id, '编辑', 'open:api:edit', NOW(), NOW()),
        (open_api_menu_id, '删除', 'open:api:delete', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 接口权限按钮权限
    IF scope_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (scope_menu_id, '查询', 'open:scope:query', NOW(), NOW()),
        (scope_menu_id, '新增', 'open:scope:add', NOW(), NOW()),
        (scope_menu_id, '编辑', 'open:scope:edit', NOW(), NOW()),
        (scope_menu_id, '删除', 'open:scope:delete', NOW(), NOW()),
        (scope_menu_id, '关联API', 'open:scope:bindApis', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 开放平台日志按钮
    IF open_log_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (open_log_menu_id, '查询', 'open:log:query', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF msg_tpl_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES
        (msg_tpl_menu_id, 'message:template:query', '查询', NOW(), NOW()),
        (msg_tpl_menu_id, 'message:template:add', '新增', NOW(), NOW()),
        (msg_tpl_menu_id, 'message:template:edit', '编辑', NOW(), NOW()),
        (msg_tpl_menu_id, 'message:template:delete', '删除', NOW(), NOW()),
        (msg_tpl_menu_id, 'message:template:test', '测试', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;
    END IF;

    IF msg_sms_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES
        (msg_sms_menu_id, 'message:send:sms', '发送', NOW(), NOW()),
        (msg_sms_menu_id, 'message:send:sms:query', '查询', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;
    END IF;

    IF msg_email_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES
        (msg_email_menu_id, 'message:send:email', '发送', NOW(), NOW()),
        (msg_email_menu_id, 'message:send:email:query', '查询', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;
    END IF;

    IF msg_internal_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES
        (msg_internal_menu_id, 'message:send:internal', '发送', NOW(), NOW()),
        (msg_internal_menu_id, 'message:send:internal:query', '查询', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;
    END IF;

    IF msg_log_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES
        (msg_log_menu_id, 'message:record:query', '查询', NOW(), NOW()),
        (msg_log_menu_id, 'message:record:detail', '详情', NOW(), NOW()),
        (msg_log_menu_id, 'message:record:retry', '重发', NOW(), NOW()),
        (msg_log_menu_id, 'message:record:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;
    END IF;

END $$;

COMMIT;
