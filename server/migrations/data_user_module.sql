-- =============================================
-- User Module - Seed Data
-- =============================================

BEGIN;

-- 1. 注册菜单
DO $$ 
DECLARE 
    manage_menu_id BIGINT; 
    user_menu_id BIGINT;
BEGIN 
    SELECT id INTO manage_menu_id FROM admin_menu WHERE route_name = 'manage' AND deleted_at = 0; 
 
    -- 用户管理子菜单 
    IF manage_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (manage_menu_id, '用户管理', 'manage_user', '/manage/user', 'view.manage_user', 'ic:round-people', 4, false, '1', '2', 'route.manage_user', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key; 
        
        SELECT id INTO user_menu_id FROM admin_menu WHERE route_name = 'manage_user' AND deleted_at = 0;
        
        -- 2. 注册 API 权限
        IF user_menu_id IS NOT NULL THEN
            INSERT INTO admin_api (menu_id, name, method, path, description, auth) VALUES
            (user_menu_id, '查询用户列表', 'GET', '/admin/v1/system/users', '获取终端用户分页列表', '1'),
            (user_menu_id, '获取用户详情', 'GET', '/admin/v1/system/users/:id', '获取单个终端用户详细信息', '1'),
            (user_menu_id, '创建用户', 'POST', '/admin/v1/system/users', '手动创建一个终端用户', '1'),
            (user_menu_id, '更新用户', 'PUT', '/admin/v1/system/users/:id', '更新终端用户信息', '1'),
            (user_menu_id, '删除用户', 'DELETE', '/admin/v1/system/users/:id', '软删除终端用户', '1'),
            (user_menu_id, '更新用户状态', 'PATCH', '/admin/v1/system/users/:id/status', '启用或禁用终端用户', '1')
            ON CONFLICT (method, path) WHERE deleted_at = 0 DO UPDATE SET name = EXCLUDED.name;

            -- 3. 注册按钮权限
            INSERT INTO admin_button (menu_id, code, label) VALUES
            (user_menu_id, 'manage_user_add', 'common.add'),
            (user_menu_id, 'manage_user_edit', 'common.edit'),
            (user_menu_id, 'manage_user_delete', 'common.delete')
            ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;
        END IF;
    END IF; 
END $$;

COMMIT;
