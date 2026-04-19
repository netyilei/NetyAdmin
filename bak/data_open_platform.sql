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
    api_menu_id BIGINT;
    scope_menu_id BIGINT;
BEGIN 
    SELECT id INTO open_menu_id FROM admin_menu WHERE route_name = 'open-platform' AND deleted_at = 0; 
 
    IF open_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (open_menu_id, '应用管理', 'open-platform_apps', '/open-platform/apps', 'view.open-platform_apps', 'ic:baseline-apps', 1, false, '1', '2', 'route.open-platform_apps', NOW(), NOW()),
        (open_menu_id, 'API管理', 'open-platform_apis', '/open-platform/apis', 'view.open-platform_apis', 'ic:baseline-api', 2, false, '1', '2', 'route.open-platform_apis', NOW(), NOW()),
        (open_menu_id, '接口权限', 'open-platform_scopes', '/open-platform/scopes', 'view.open-platform_scopes', 'ic:baseline-security', 3, false, '1', '2', 'route.open-platform_scopes', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET 
            name = EXCLUDED.name,
            icon = EXCLUDED.icon,
            order_by = EXCLUDED.order_by,
            component = EXCLUDED.component,
            i18_n_key = EXCLUDED.i18_n_key,
            updated_at = NOW(); 
    END IF; 

    SELECT id INTO app_menu_id FROM admin_menu WHERE route_name = 'open-platform_apps' AND deleted_at = 0;
    SELECT id INTO api_menu_id FROM admin_menu WHERE route_name = 'open-platform_apis' AND deleted_at = 0;
    SELECT id INTO scope_menu_id FROM admin_menu WHERE route_name = 'open-platform_scopes' AND deleted_at = 0;

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

    -- API管理 API 初始化
    IF api_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (api_menu_id, '获取API列表', 'GET', '/admin/v1/open/apis', '获取API列表', '1', NOW(), NOW()),
        (api_menu_id, '新增API', 'POST', '/admin/v1/open/apis', '新增API', '1', NOW(), NOW()),
        (api_menu_id, '修改API', 'PUT', '/admin/v1/open/apis', '修改API', '1', NOW(), NOW()),
        (api_menu_id, '删除API', 'DELETE', '/admin/v1/open/apis', '删除API', '1', NOW(), NOW()),
        (api_menu_id, '获取全部API', 'GET', '/admin/v1/open/apis/all', '获取全部API', '1', NOW(), NOW()),
        (api_menu_id, '获取权限关联API', 'GET', '/admin/v1/open/apis/scope-apis', '获取权限关联API', '1', NOW(), NOW()),
        (api_menu_id, '更新权限关联API', 'PUT', '/admin/v1/open/apis/scope-apis', '更新权限关联API', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (api_menu_id, '查询', 'open:api:query', NOW(), NOW()),
        (api_menu_id, '新增', 'open:api:add', NOW(), NOW()),
        (api_menu_id, '编辑', 'open:api:edit', NOW(), NOW()),
        (api_menu_id, '删除', 'open:api:delete', NOW(), NOW())
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
        (scope_menu_id, '删除', 'open:scope:delete', NOW(), NOW()),
        (scope_menu_id, '关联API', 'open:scope:bindApis', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;

-- 默认测试应用种子数据
-- 注意: app_secret 为占位密文，管理员需通过「重置密钥」功能获取真实密钥
INSERT INTO sys_apps (id, app_key, app_secret, name, status, ip_strategy, remark) VALUES
('01JQDEFAULTAPP001', '01JQDEFAULTAPP001', 'placeholder-reset-secret-required', '默认测试应用', 1, 1, '系统自动创建的测试应用，请重置密钥后使用')
ON CONFLICT (app_key) WHERE deleted_at = 0 DO NOTHING;

-- 为默认应用分配全部权限
INSERT INTO sys_app_scopes (app_id, scope) VALUES
('01JQDEFAULTAPP001', 'user_base'),
('01JQDEFAULTAPP001', 'user_profile'),
('01JQDEFAULTAPP001', 'msg_send'),
('01JQDEFAULTAPP001', 'content_view')
ON CONFLICT (app_id, scope) DO NOTHING;

COMMIT;
