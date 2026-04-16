-- =============================================
-- Message Hub Module - Data
-- =============================================

BEGIN;

-- 核心菜单初始化
INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
VALUES 
(0, '消息管理', 'message-hub', '/message', 'layout', 'ic:baseline-message', 6, false, '1', '1', 'route.message_hub', NOW(), NOW())
ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key;

-- 模块子菜单初始化 
DO $$ 
DECLARE 
    msg_menu_id BIGINT; 
BEGIN 
    SELECT id INTO msg_menu_id FROM admin_menu WHERE route_name = 'message-hub' AND deleted_at = 0; 
 
    IF msg_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (msg_menu_id, '消息模板', 'msg_template', '/message/template', 'view.msg_template', 'ic:baseline-template', 1, false, '1', '2', 'route.message_template', NOW(), NOW()),
        (msg_menu_id, '发送记录', 'msg_record', '/message/record', 'view.msg_record', 'ic:baseline-history', 2, false, '1', '2', 'route.message_log', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key, component = EXCLUDED.component; 
    END IF; 
END $$;

-- 消息模板 API 与按钮权限初始化
DO $$ 
DECLARE 
    tpl_menu_id BIGINT; 
BEGIN 
    SELECT id INTO tpl_menu_id FROM admin_menu WHERE route_name = 'msg_template' AND deleted_at = 0; 
 
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
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;

-- 发送记录 API 权限初始化
DO $$ 
DECLARE 
    rec_menu_id BIGINT; 
BEGIN 
    SELECT id INTO rec_menu_id FROM admin_menu WHERE route_name = 'msg_record' AND deleted_at = 0; 
 
    IF rec_menu_id IS NOT NULL THEN 
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (rec_menu_id, '获取记录列表', 'GET', '/admin/v1/message/records', '获取记录列表', '1', NOW(), NOW()),
        (rec_menu_id, '直接发送消息', 'POST', '/admin/v1/message/send', '管理员发送消息', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (rec_menu_id, '查询', 'msg:record:query', NOW(), NOW()),
        (rec_menu_id, '发送', 'msg:record:send', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;

COMMIT;
