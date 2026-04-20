BEGIN;

-- 1. 新增邮件配置项: SSL 开关 + 认证方式
INSERT INTO sys_configs (group_name, config_key, config_value, value_type, description, is_system, created_by, updated_by)
VALUES
('email_config', 'ssl_enabled', 'true', 'boolean', '是否启用 SSL/TLS 加密连接', FALSE, 1, 1),
('email_config', 'auth_type', 'plain', 'string', 'SMTP 认证方式 (plain/crammd5)', FALSE, 1, 1)
ON CONFLICT (group_name, config_key) WHERE deleted_at = 0 DO UPDATE SET
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

-- 2. 新增测试邮件 API + 按钮
DO $$
DECLARE
    config_menu_id BIGINT;
    super_role_id BIGINT;
    new_api_id BIGINT;
    new_button_id BIGINT;
BEGIN
    SELECT id INTO config_menu_id FROM admin_menu WHERE route_name = 'manage_system_setting' AND deleted_at = 0;

    -- 新增测试邮件 API
    IF config_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES
        (config_menu_id, '测试邮件发送', 'POST', '/admin/v1/system/test-email', '测试邮件发送', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        -- 新增测试邮件按钮
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES
        (config_menu_id, 'email:test', '测试邮件', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;

        -- 将新增 API 和按钮关联到超级管理员角色
        SELECT id INTO super_role_id FROM admin_role WHERE code = 'super_admin' AND deleted_at = 0;

        IF super_role_id IS NOT NULL THEN
            SELECT id INTO new_api_id FROM admin_api WHERE method = 'POST' AND path = '/admin/v1/system/test-email' AND deleted_at = 0;
            IF new_api_id IS NOT NULL THEN
                INSERT INTO admin_role_apis (role_id, api_id, created_at, updated_at)
                VALUES (super_role_id, new_api_id, NOW(), NOW())
                ON CONFLICT (role_id, api_id) WHERE deleted_at = 0 DO NOTHING;
            END IF;

            SELECT id INTO new_button_id FROM admin_button WHERE code = 'email:test' AND deleted_at = 0;
            IF new_button_id IS NOT NULL THEN
                INSERT INTO admin_role_buttons (role_id, button_id, created_at, updated_at)
                VALUES (super_role_id, new_button_id, NOW(), NOW())
                ON CONFLICT (role_id, button_id) WHERE deleted_at = 0 DO NOTHING;
            END IF;
        END IF;
    END IF;
END $$;

COMMIT;
