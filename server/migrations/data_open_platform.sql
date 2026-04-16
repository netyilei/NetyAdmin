-- =============================================
-- Open Platform Module - Data
-- =============================================

BEGIN;

-- 核心菜单初始化
INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
VALUES 
(0, '开放平台', 'open-platform', '/open-platform', 'layout', 'ic:outline-settings-input-component', 5, false, '1', '1', 'route.open_platform', NOW(), NOW())
ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key;

-- 模块子菜单初始化 
DO $$ 
DECLARE 
    open_menu_id BIGINT; 
    app_menu_id BIGINT;
    scope_menu_id BIGINT;
BEGIN 
    SELECT id INTO open_menu_id FROM admin_menu WHERE route_name = 'open-platform' AND deleted_at = 0; 
 
    -- 开放平台子菜单: 应用管理
    IF open_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (open_menu_id, '应用管理', 'open_app', '/open-platform/apps', 'view.open_app', 'ic:baseline-apps', 1, false, '1', '2', 'route.open_platform_apps', NOW(), NOW()),
        (open_menu_id, '接口权限', 'open_scope', '/open-platform/scopes', 'view.open_scope', 'ic:baseline-security', 2, false, '1', '2', 'route.open_platform_scopes', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key, component = EXCLUDED.component; 
    END IF; 

    SELECT id INTO app_menu_id FROM admin_menu WHERE route_name = 'open_app' AND deleted_at = 0;
    SELECT id INTO scope_menu_id FROM admin_menu WHERE route_name = 'open_scope' AND deleted_at = 0;

    -- 应用管理 API 初始化
    IF app_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (app_menu_id, '获取应用列表', 'GET', '/admin/v1/open/apps', '获取应用列表', '1', NOW(), NOW()),
        (app_menu_id, '新增应用', 'POST', '/admin/v1/open/apps', '新增应用', '1', NOW(), NOW()),
        (app_menu_id, '修改应用', 'PUT', '/admin/v1/open/apps', '修改应用', '1', NOW(), NOW()),
        (app_menu_id, '删除应用', 'DELETE', '/admin/v1/open/apps', '删除应用', '1', NOW(), NOW()),
        (app_menu_id, '重置 AppSecret', 'PUT', '/admin/v1/open/apps/reset-secret', '重置 AppSecret', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        -- 按钮权限
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (app_menu_id, '查询', 'open:app:query', NOW(), NOW()),
        (app_menu_id, '新增', 'open:app:add', NOW(), NOW()),
        (app_menu_id, '编辑', 'open:app:edit', NOW(), NOW()),
        (app_menu_id, '重置密钥', 'open:app:resetSecret', NOW(), NOW()),
        (app_menu_id, '删除', 'open:app:delete', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 接口权限 API 初始化
    IF scope_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (scope_menu_id, '获取权限列表', 'GET', '/admin/v1/open/scopes', '获取权限列表', '1', NOW(), NOW()),
        (scope_menu_id, '新增权限', 'POST', '/admin/v1/open/scopes', '新增权限', '1', NOW(), NOW()),
        (scope_menu_id, '修改权限', 'PUT', '/admin/v1/open/scopes', '修改权限', '1', NOW(), NOW()),
        (scope_menu_id, '删除权限', 'DELETE', '/admin/v1/open/scopes', '删除权限', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        -- 按钮权限
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (scope_menu_id, '查询', 'open:scope:query', NOW(), NOW()),
        (scope_menu_id, '新增', 'open:scope:add', NOW(), NOW()),
        (scope_menu_id, '编辑', 'open:scope:edit', NOW(), NOW()),
        (scope_menu_id, '删除', 'open:scope:delete', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;

COMMIT;
