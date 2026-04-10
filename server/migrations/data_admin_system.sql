-- =============================================
-- Admin System Module - Data
-- =============================================

-- 核心菜单初始化
INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
SELECT 0, '首页', 'home', '/home', 'view.home', 'mdi:monitor-dashboard', 0, false, '1', '2', 'route.home', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'home');

INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
SELECT 0, '管理员', 'manage', '/manage', 'layout', 'ic:outline-settings', 1, false, '1', '1', 'route.manage', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'manage');

INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
SELECT 0, '运维', 'ops', '/ops', 'layout', 'ic:outline-build', 2, false, '1', '1', 'route.ops', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'ops');

INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
SELECT 0, '基础设置', 'settings', '/settings', 'layout', 'ic:outline-settings', 3, false, '1', '1', 'route.settings', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'settings');

INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
SELECT 0, '内容管理', 'content', '/content', 'layout', 'ic:outline-article', 4, false, '1', '1', 'route.content', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'content');

-- 模块子菜单初始化
DO $$
DECLARE
    manage_menu_id BIGINT;
    ops_menu_id BIGINT;
    settings_menu_id BIGINT;
BEGIN
    SELECT id INTO manage_menu_id FROM admin_menu WHERE route_name = 'manage';
    SELECT id INTO ops_menu_id FROM admin_menu WHERE route_name = 'ops';
    SELECT id INTO settings_menu_id FROM admin_menu WHERE route_name = 'settings';

    -- 管理员管理子菜单
    IF manage_menu_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        SELECT manage_menu_id, '管理员管理', 'manage_admin', '/manage/admin', 'view.manage_admin', 'ic:round-manage-accounts', 1, false, '1', '2', 'route.manage_admin', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'manage_admin');

        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        SELECT manage_menu_id, '角色管理', 'manage_role', '/manage/role', 'view.manage_role', 'carbon:user-role', 2, false, '1', '2', 'route.manage_role', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'manage_role');

        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        SELECT manage_menu_id, '菜单管理', 'manage_menu', '/manage/menu', 'view.manage_menu', 'material-symbols:route', 3, false, '1', '2', 'route.manage_menu', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'manage_menu');
    END IF;

    -- 运维管理子菜单
    IF ops_menu_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        SELECT ops_menu_id, '操作日志', 'ops_operation-log', '/ops/operation-log', 'view.ops_operation-log', 'ic:outline-history', 1, false, '1', '2', 'route.ops_operation-log', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'ops_operation-log');

        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        SELECT ops_menu_id, '错误日志', 'ops_error-log', '/ops/error-log', 'view.ops_error-log', 'ic:outline-error-outline', 2, false, '1', '2', 'route.ops_error-log', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'ops_error-log');

        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        SELECT ops_menu_id, '任务调度', 'ops_task', '/ops/task', 'view.ops_task', 'ic:outline-schedule', 3, false, '1', '2', 'route.ops_task', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'ops_task');
    END IF;

    -- 基础设置子菜单
    IF settings_menu_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        SELECT settings_menu_id, '基础设置', 'manage_system_setting', '/manage/system/setting', 'view.manage_system_setting', 'carbon:settings-adjust', 1, false, '1', '2', 'route.manage_system_setting', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'manage_system_setting');

        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        SELECT settings_menu_id, '字典管理', 'manage_dict', '/manage/dict', 'view.manage_dict', 'mdi:book-open-variant', 2, false, '1', '2', 'route.manage_dict', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'manage_dict');
    END IF;
END $$;

-- 系统核心API初始化
DO $$
DECLARE
    admin_menu_id BIGINT;
    role_menu_id BIGINT;
    menu_menu_id BIGINT;
    op_log_menu_id BIGINT;
    err_log_menu_id BIGINT;
    task_menu_id BIGINT;
    config_menu_id BIGINT;
BEGIN
    SELECT id INTO admin_menu_id FROM admin_menu WHERE route_name = 'manage_admin';
    SELECT id INTO role_menu_id FROM admin_menu WHERE route_name = 'manage_role';
    SELECT id INTO menu_menu_id FROM admin_menu WHERE route_name = 'manage_menu';
    SELECT id INTO op_log_menu_id FROM admin_menu WHERE route_name = 'ops_operation-log';
    SELECT id INTO err_log_menu_id FROM admin_menu WHERE route_name = 'ops_error-log';
    SELECT id INTO task_menu_id FROM admin_menu WHERE route_name = 'ops_task';
    SELECT id INTO config_menu_id FROM admin_menu WHERE route_name = 'manage_system_setting';

    -- 用户管理API
    IF admin_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (admin_menu_id, '获取管理员列表', 'GET', '/admin/v1/admins', '获取管理员列表', '1', NOW(), NOW()),
        (admin_menu_id, '获取管理员详情', 'GET', '/admin/v1/admins/:id', '获取管理员详情', '1', NOW(), NOW()),
        (admin_menu_id, '创建管理员', 'POST', '/admin/v1/admins', '创建管理员', '1', NOW(), NOW()),
        (admin_menu_id, '更新管理员', 'PUT', '/admin/v1/admins/:id', '更新管理员', '1', NOW(), NOW()),
        (admin_menu_id, '删除管理员', 'DELETE', '/admin/v1/admins/:id', '删除管理员', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
    END IF;

    -- 角色管理API
    IF role_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (role_menu_id, '获取角色列表', 'GET', '/admin/v1/systemManage/getRoleList', '获取角色列表', '1', NOW(), NOW()),
        (role_menu_id, '获取角色详情', 'GET', '/admin/v1/systemManage/getRole/:id', '获取角色详情', '1', NOW(), NOW()),
        (role_menu_id, '获取所有角色', 'GET', '/admin/v1/systemManage/getAllRoles', '获取所有角色', '1', NOW(), NOW()),
        (role_menu_id, '添加角色', 'POST', '/admin/v1/systemManage/addRole', '添加角色', '1', NOW(), NOW()),
        (role_menu_id, '更新角色', 'PUT', '/admin/v1/systemManage/updateRole', '更新角色', '1', NOW(), NOW()),
        (role_menu_id, '删除角色', 'DELETE', '/admin/v1/systemManage/deleteRole', '删除角色', '1', NOW(), NOW()),
        (role_menu_id, '获取角色API', 'GET', '/admin/v1/systemManage/role/:id/apis', '获取角色API', '1', NOW(), NOW()),
        (role_menu_id, '更新角色API', 'PUT', '/admin/v1/systemManage/role/:id/apis', '更新角色API', '1', NOW(), NOW()),
        (role_menu_id, '获取角色按钮', 'GET', '/admin/v1/systemManage/role/:id/buttons', '获取角色按钮', '1', NOW(), NOW()),
        (role_menu_id, '更新角色按钮', 'PUT', '/admin/v1/systemManage/role/:id/buttons', '更新角色按钮', '1', NOW(), NOW()),
        (role_menu_id, '获取角色菜单', 'GET', '/admin/v1/systemManage/role/:id/menus', '获取角色菜单', '1', NOW(), NOW()),
        (role_menu_id, '更新角色菜单', 'PUT', '/admin/v1/systemManage/role/:id/menus', '更新角色菜单', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
    END IF;

    -- 菜单管理API
    IF menu_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (menu_menu_id, '获取菜单树', 'GET', '/admin/v1/systemManage/getMenuTree', '获取菜单树', '1', NOW(), NOW()),
        (menu_menu_id, '获取按钮授权树', 'GET', '/admin/v1/systemManage/getButtonTree', '获取角色授权时的按钮权限树', '1', NOW(), NOW()),
        (menu_menu_id, '获取API授权树', 'GET', '/admin/v1/systemManage/getApiTree', '获取角色授权时的API权限树', '1', NOW(), NOW()),
        (menu_menu_id, '添加菜单', 'POST', '/admin/v1/systemManage/addMenu', '添加菜单', '1', NOW(), NOW()),
        (menu_menu_id, '更新菜单', 'PUT', '/admin/v1/systemManage/updateMenu', '更新菜单', '1', NOW(), NOW()),
        (menu_menu_id, '删除菜单', 'DELETE', '/admin/v1/systemManage/deleteMenu', '删除菜单', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
    END IF;

    -- 运维日志API
    IF op_log_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (op_log_menu_id, '获取操作日志列表', 'GET', '/admin/v1/operation-logs', '获取操作日志列表', '1', NOW(), NOW()),
        (op_log_menu_id, '删除操作日志', 'DELETE', '/admin/v1/operation-logs/:id', '删除操作日志', '1', NOW(), NOW()),
        (op_log_menu_id, '批量删除操作日志', 'POST', '/admin/v1/operation-logs/batch-delete', '批量删除操作日志', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
    END IF;

    -- 任务调度API
    IF task_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (task_menu_id, '获取任务列表', 'GET', '/admin/v1/system/tasks', '获取后端任务列表', '1', NOW(), NOW()),
        (task_menu_id, '执行后台任务', 'POST', '/admin/v1/system/tasks/:name/run', '手动触发任务执行一次', '1', NOW(), NOW()),
        (task_menu_id, '启动后台任务', 'POST', '/admin/v1/system/tasks/:name/start', '启动指定的后台调度任务', '1', NOW(), NOW()),
        (task_menu_id, '停止后台任务', 'POST', '/admin/v1/system/tasks/:name/stop', '停止运行中的后台调度任务', '1', NOW(), NOW()),
        (task_menu_id, '重启后台任务', 'POST', '/admin/v1/system/tasks/:name/reload', '重新加载并启动任务', '1', NOW(), NOW()),
        (task_menu_id, '修改任务配置', 'PUT', '/admin/v1/system/tasks/:name', '修改任务执行周期或启用状态', '1', NOW(), NOW()),
        (task_menu_id, '查询任务日志', 'GET', '/admin/v1/system/tasks/logs', '查询后台任务执行历史', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
    END IF;

    IF err_log_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (err_log_menu_id, '获取错误日志列表', 'GET', '/admin/v1/error-logs', '获取错误日志列表', '1', NOW(), NOW()),
        (err_log_menu_id, '解决错误日志', 'PUT', '/admin/v1/error-logs/:id/resolve', '解决错误日志', '1', NOW(), NOW()),
        (err_log_menu_id, '删除错误日志', 'DELETE', '/admin/v1/error-logs/:id', '删除错误日志', '1', NOW(), NOW()),
        (err_log_menu_id, '批量删除错误日志', 'POST', '/admin/v1/error-logs/batch-delete', '批量删除错误日志', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
    END IF;

    -- 系统配置API
    IF config_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (config_menu_id, '查询系统配置列表', 'GET', '/admin/v1/system/configs', '查询系统配置列表', '1', NOW(), NOW()),
        (config_menu_id, '修改系统配置及热更新', 'PUT', '/admin/v1/system/configs', '修改系统配置及热更新', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
    END IF;

    -- 字典管理API
    DECLARE
        dict_menu_id BIGINT;
    BEGIN
        SELECT id INTO dict_menu_id FROM admin_menu WHERE route_name = 'manage_dict';
        IF dict_menu_id IS NOT NULL THEN
            INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
            VALUES 
            (dict_menu_id, '获取字典类型列表', 'GET', '/admin/v1/system/dict/types', '获取字典类型管理列表', '1', NOW(), NOW()),
            (dict_menu_id, '创建字典类型', 'POST', '/admin/v1/system/dict/types', '创建新的字典类型', '1', NOW(), NOW()),
            (dict_menu_id, '更新字典类型', 'PUT', '/admin/v1/system/dict/types', '更新现有字典类型', '1', NOW(), NOW()),
            (dict_menu_id, '删除字典类型', 'DELETE', '/admin/v1/system/dict/types/:id', '删除字典类型', '1', NOW(), NOW()),
            (dict_menu_id, '获取字典数据列表', 'GET', '/admin/v1/system/dict/data', '获取字典数据管理列表', '1', NOW(), NOW()),
            (dict_menu_id, '获取字典数据详情', 'GET', '/admin/v1/system/dict/data/:code', '前端带缓存获取字典数据', '1', NOW(), NOW()),
            (dict_menu_id, '创建字典数据', 'POST', '/admin/v1/system/dict/data', '创建新的字典数据项', '1', NOW(), NOW()),
            (dict_menu_id, '更新字典数据', 'PUT', '/admin/v1/system/dict/data', '更新字典数据项', '1', NOW(), NOW()),
            (dict_menu_id, '删除字典数据', 'DELETE', '/admin/v1/system/dict/data/:id', '删除字典数据项', '1', NOW(), NOW())
            ON CONFLICT (method, path) DO NOTHING;
        END IF;
    END;
END $$;

-- 系统核心按钮初始化
DO $$
DECLARE
    admin_menu_id BIGINT;
    role_menu_id BIGINT;
    menu_menu_id BIGINT;
BEGIN
    SELECT id INTO admin_menu_id FROM admin_menu WHERE route_name = 'manage_admin';
    SELECT id INTO role_menu_id FROM admin_menu WHERE route_name = 'manage_role';
    SELECT id INTO menu_menu_id FROM admin_menu WHERE route_name = 'manage_menu';

    -- 管理员管理按钮
    IF admin_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (admin_menu_id, 'user:add', '新增', NOW(), NOW()),
        (admin_menu_id, 'user:edit', '编辑', NOW(), NOW()),
        (admin_menu_id, 'user:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) DO NOTHING;
    END IF;

    -- 角色管理按钮
    IF role_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (role_menu_id, 'role:add', '新增', NOW(), NOW()),
        (role_menu_id, 'role:edit', '编辑', NOW(), NOW()),
        (role_menu_id, 'role:delete', '删除', NOW(), NOW()),
        (role_menu_id, 'role:auth', '授权', NOW(), NOW())
        ON CONFLICT (code) DO NOTHING;
    END IF;

    -- 菜单管理按钮
    IF menu_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (menu_menu_id, 'menu:add', '新增', NOW(), NOW()),
        (menu_menu_id, 'menu:edit', '编辑', NOW(), NOW()),
        (menu_menu_id, 'menu:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) DO NOTHING;
    END IF;

    -- 字典管理按钮
    DECLARE
        dict_menu_id BIGINT;
    BEGIN
        SELECT id INTO dict_menu_id FROM admin_menu WHERE route_name = 'manage_dict';
        IF dict_menu_id IS NOT NULL THEN
            INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
            VALUES 
            (dict_menu_id, 'dict:add', '新增', NOW(), NOW()),
            (dict_menu_id, 'dict:edit', '编辑', NOW(), NOW()),
            (dict_menu_id, 'dict:delete', '删除', NOW(), NOW())
            ON CONFLICT (code) DO NOTHING;
        END IF;
    END;
END $$;

-- 默认超级管理员及权限分配
INSERT INTO admin_role (name, code, description, status)
SELECT '超级管理员', 'R_SUPER', '系统顶级角色', '1'
WHERE NOT EXISTS (SELECT 1 FROM admin_role WHERE code = 'R_SUPER');

INSERT INTO admin_user (username, password, nickname, status)
SELECT 'admin', '$2a$10$QJgEZIAq8nlUi8jRftfKtu.RQ0Z8CV/YNBRIpOBA9YPJ7HaB5uj8C', '超级管理员', '1'
WHERE NOT EXISTS (SELECT 1 FROM admin_user WHERE username = 'admin');

-- 关联用户与角色
INSERT INTO admin_user_roles (admin_user_id, admin_role_id)
SELECT u.id, r.id FROM admin_user u, admin_role r 
WHERE u.username = 'admin' AND r.code = 'R_SUPER'
ON CONFLICT DO NOTHING;

-- 自动授权逻辑：为超级管理员分配所有现有权限
DO $$
DECLARE
    super_role_id BIGINT;
BEGIN
    SELECT id INTO super_role_id FROM admin_role WHERE code = 'R_SUPER';

    IF super_role_id IS NOT NULL THEN
        -- 分配所有菜单
        INSERT INTO admin_role_menus (admin_role_id, admin_menu_id)
        SELECT super_role_id, id FROM admin_menu
        ON CONFLICT DO NOTHING;

        -- 分配所有API
        INSERT INTO admin_role_apis (admin_role_id, admin_api_id)
        SELECT super_role_id, id FROM admin_api
        ON CONFLICT DO NOTHING;

        -- 分配所有按钮
        INSERT INTO admin_role_buttons (admin_role_id, admin_button_id)
        SELECT super_role_id, id FROM admin_button
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- 系统核心配置初始化
INSERT INTO sys_configs (group_name, config_key, config_value, value_type, description, is_system, created_at, updated_at)
VALUES 
('cache_switches', 'err_log_cache', 'true', 'string', '错误日志缓存开关', FALSE, NOW(), NOW()),
('cache_switches', 'content_category_cache', 'true', 'string', '内容分类树缓存开关', FALSE, NOW(), NOW())
ON CONFLICT (group_name, config_key) DO UPDATE SET 
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

-- 重置主键序列
SELECT setval(pg_get_serial_sequence('admin_user', 'id'), (SELECT COALESCE(MAX(id), 1) FROM admin_user));
SELECT setval(pg_get_serial_sequence('admin_role', 'id'), (SELECT COALESCE(MAX(id), 1) FROM admin_role));
SELECT setval(pg_get_serial_sequence('admin_menu', 'id'), (SELECT COALESCE(MAX(id), 1) FROM admin_menu));
SELECT setval(pg_get_serial_sequence('admin_api', 'id'), (SELECT COALESCE(MAX(id), 1) FROM admin_api));
SELECT setval(pg_get_serial_sequence('admin_button', 'id'), (SELECT COALESCE(MAX(id), 1) FROM admin_button));
