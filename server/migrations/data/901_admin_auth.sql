BEGIN;

-- 默认超级管理员及权限分配
INSERT INTO admin_role (name, code, description, status)
SELECT '超级管理员', 'R_SUPER', '系统顶级角色', '1'
WHERE NOT EXISTS (SELECT 1 FROM admin_role WHERE code = 'R_SUPER' AND deleted_at = 0);

INSERT INTO admin_role (name, code, description, status)
SELECT '管理员', 'R_ADMIN', '普通管理员，拥有部分权限', '1'
WHERE NOT EXISTS (SELECT 1 FROM admin_role WHERE code = 'R_ADMIN' AND deleted_at = 0);

INSERT INTO admin_user (username, password, nickname, status, created_at, updated_at)
SELECT 'admin', '$2a$10$QJgEZIAq8nlUi8jRftfKtu.RQ0Z8CV/YNBRIpOBA9YPJ7HaB5uj8C', '超级管理员', '1', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM admin_user WHERE username = 'admin' AND deleted_at = 0);

-- 关联用户与角色
INSERT INTO admin_user_roles (admin_user_id, admin_role_id)
SELECT u.id, r.id FROM admin_user u, admin_role r 
WHERE u.username = 'admin' AND r.code = 'R_SUPER' AND u.deleted_at = 0 AND r.deleted_at = 0
ON CONFLICT (admin_user_id, admin_role_id) DO NOTHING;

-- 自动授权逻辑：为超级管理员分配所有现有权限
DO $$
DECLARE
    super_role_id BIGINT;
BEGIN
    SELECT id INTO super_role_id FROM admin_role WHERE code = 'R_SUPER' AND deleted_at = 0;

    IF super_role_id IS NOT NULL THEN
        -- 分配所有菜单
        INSERT INTO admin_role_menus (admin_role_id, admin_menu_id)
        SELECT super_role_id, id FROM admin_menu WHERE deleted_at = 0
        ON CONFLICT (admin_role_id, admin_menu_id) DO NOTHING;

        -- 分配所有API
        INSERT INTO admin_role_apis (admin_role_id, admin_api_id)
        SELECT super_role_id, id FROM admin_api WHERE deleted_at = 0
        ON CONFLICT (admin_role_id, admin_api_id) DO NOTHING;

        -- 分配所有按钮
        INSERT INTO admin_role_buttons (admin_role_id, admin_button_id)
        SELECT super_role_id, id FROM admin_button WHERE deleted_at = 0
        ON CONFLICT (admin_role_id, admin_button_id) DO NOTHING;
    END IF;
END $$;

COMMIT;