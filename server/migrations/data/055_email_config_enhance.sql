BEGIN;

DO $$
DECLARE
    config_menu_id BIGINT;
    super_role_id BIGINT;
    new_api_id BIGINT;
    new_button_id BIGINT;
BEGIN
    SELECT id INTO config_menu_id FROM admin_menu WHERE route_name = 'manage_system_setting' AND deleted_at = 0;

    IF config_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES
        (config_menu_id, '测试邮件发送', 'POST', '/admin/v1/system/test-email', '测试邮件发送', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES
        (config_menu_id, 'email:test', '测试邮件', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;

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
