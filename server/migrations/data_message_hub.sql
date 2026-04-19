-- =============================================
-- Message Hub Module - Data
-- =============================================

BEGIN;

-- 核心菜单初始化
INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
VALUES 
(0, '消息', 'message', '/message', 'layout', 'ic:baseline-message', 6, false, '1', '1', 'route.message', NOW(), NOW())
ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key;

-- 模块子菜单初始化 
DO $$ 
DECLARE 
    msg_menu_id BIGINT; 
    ops_menu_id BIGINT;
    settings_menu_id BIGINT;
BEGIN 
    SELECT id INTO msg_menu_id FROM admin_menu WHERE route_name = 'message' AND deleted_at = 0; 
    SELECT id INTO ops_menu_id FROM admin_menu WHERE route_name = 'ops' AND deleted_at = 0;
    SELECT id INTO settings_menu_id FROM admin_menu WHERE route_name = 'settings' AND deleted_at = 0;
 
    IF msg_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (msg_menu_id, '发送短信', 'message_send-sms', '/message/send-sms', 'view.message_send-sms', 'ic:baseline-sms', 1, false, '1', '2', 'route.message_send_sms', NOW(), NOW()),
        (msg_menu_id, '发送邮件', 'message_send-email', '/message/send-email', 'view.message_send-email', 'ic:baseline-email', 2, false, '1', '2', 'route.message_send_email', NOW(), NOW()),
        (msg_menu_id, '发送站内信', 'message_send-internal', '/message/send-internal', 'view.message_send-internal', 'ic:baseline-notifications', 3, false, '1', '2', 'route.message_send_internal', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET 
            name = EXCLUDED.name,
            icon = EXCLUDED.icon,
            order_by = EXCLUDED.order_by,
            component = EXCLUDED.component,
            i18_n_key = EXCLUDED.i18_n_key,
            updated_at = NOW(); 
    END IF; 

    IF settings_menu_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (settings_menu_id, '消息模板', 'settings_message-template', '/settings/message-template', 'view.settings_message-template', 'mdi:file-document-edit-outline', 3, false, '1', '2', 'route.settings_message-template', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET 
            name = EXCLUDED.name,
            icon = EXCLUDED.icon,
            order_by = EXCLUDED.order_by,
            component = EXCLUDED.component,
            parent_id = EXCLUDED.parent_id,
            route_path = EXCLUDED.route_path,
            i18_n_key = EXCLUDED.i18_n_key,
            updated_at = NOW();
    END IF;

    IF ops_menu_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (ops_menu_id, '消息记录', 'ops_message-log', '/ops/message-log', 'view.ops_message-log', 'ic:baseline-history', 5, false, '1', '2', 'route.ops_message-log', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET 
            name = EXCLUDED.name,
            icon = EXCLUDED.icon,
            order_by = EXCLUDED.order_by,
            component = EXCLUDED.component,
            i18_n_key = EXCLUDED.i18_n_key,
            updated_at = NOW();
    END IF;
END $$;

-- 消息模板 API 与按钮权限初始化
DO $$ 
DECLARE 
    tpl_menu_id BIGINT; 
    sms_menu_id BIGINT;
    email_menu_id BIGINT;
    internal_menu_id BIGINT;
BEGIN 
    SELECT id INTO tpl_menu_id FROM admin_menu WHERE route_name = 'settings_message-template' AND deleted_at = 0; 
    SELECT id INTO sms_menu_id FROM admin_menu WHERE route_name = 'message_send-sms' AND deleted_at = 0;
    SELECT id INTO email_menu_id FROM admin_menu WHERE route_name = 'message_send-email' AND deleted_at = 0;
    SELECT id INTO internal_menu_id FROM admin_menu WHERE route_name = 'message_send-internal' AND deleted_at = 0;
 
    IF tpl_menu_id IS NOT NULL THEN 
        -- API 权限
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (tpl_menu_id, '获取模板列表', 'GET', '/admin/v1/message/templates', '获取模板列表', '1', NOW(), NOW()),
        (tpl_menu_id, '新增模板', 'POST', '/admin/v1/message/templates', '新增模板', '1', NOW(), NOW()),
        (tpl_menu_id, '修改模板', 'PUT', '/admin/v1/message/templates', '修改模板', '1', NOW(), NOW()),
        (tpl_menu_id, '删除模板', 'DELETE', '/admin/v1/message/templates/:id', '删除模板', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        -- 按钮权限
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (tpl_menu_id, '查询', 'msg:template:query', NOW(), NOW()),
        (tpl_menu_id, '新增', 'msg:template:add', NOW(), NOW()),
        (tpl_menu_id, '编辑', 'msg:template:edit', NOW(), NOW()),
        (tpl_menu_id, '删除', 'msg:template:delete', NOW(), NOW()),
        (tpl_menu_id, '测试', 'msg:template:test', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;
    END IF;

    IF sms_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (sms_menu_id, '发送短信', 'POST', '/admin/v1/message/send', '管理员发送短信', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES (sms_menu_id, '发送', 'msg:send:sms', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF email_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (email_menu_id, '发送邮件', 'POST', '/admin/v1/message/send', '管理员发送邮件', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES (email_menu_id, '发送', 'msg:send:email', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF internal_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (internal_menu_id, '发送站内信', 'POST', '/admin/v1/message/send', '管理员发送站内信', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES (internal_menu_id, '发送', 'msg:send:internal', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;

-- 发送记录 API 权限初始化
DO $$ 
DECLARE 
    rec_menu_id BIGINT; 
BEGIN 
    SELECT id INTO rec_menu_id FROM admin_menu WHERE route_name = 'ops_message-log' AND deleted_at = 0; 
 
    IF rec_menu_id IS NOT NULL THEN 
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (rec_menu_id, '获取记录列表', 'GET', '/admin/v1/message/records', '获取记录列表', '1', NOW(), NOW()),
        (rec_menu_id, '失败重发', 'POST', '/admin/v1/message/records/:id/retry', '失败消息重发', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (rec_menu_id, '查询', 'msg:log:query', NOW(), NOW()),
        (rec_menu_id, '详情', 'msg:log:detail', NOW(), NOW()),
        (rec_menu_id, '重发', 'msg:log:retry', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;
    END IF;
END $$;

-- 默认消息模板种子数据
INSERT INTO msg_templates (code, name, channel, title, content, status) VALUES
('verify_code_sms', '短信验证码', 'sms', NULL, '您的验证码为：{{.code}}，有效期{{.expire}}分钟，请勿泄露。', 1),
('verify_code_email', '邮箱验证码', 'email', '验证码', '您的验证码为：{{.code}}，有效期{{.expire}}分钟，请勿泄露。', 1),
('welcome_internal', '欢迎注册', 'internal', '欢迎加入', '亲爱的{{.nickname}}，欢迎注册！', 1),
('reset_password_email', '重置密码', 'email', '重置密码通知', '您正在重置密码，验证码为：{{.code}}，有效期{{.expire}}分钟。如非本人操作请忽略。', 1),
('system_notice_internal', '系统公告', 'internal', '系统公告', '{{.content}}', 1)
ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;

COMMIT;
