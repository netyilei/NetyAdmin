BEGIN;

-- 系统核心API初始化
DO $$
DECLARE
        admin_menu_id BIGINT;
        role_menu_id BIGINT;
        menu_menu_id BIGINT;
        button_menu_id BIGINT;
        api_menu_id BIGINT;
        op_log_menu_id BIGINT;
        err_log_menu_id BIGINT;
        task_menu_id BIGINT;
        config_menu_id BIGINT;
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
        SELECT id INTO op_log_menu_id FROM admin_menu WHERE route_name = 'ops_operation-log' AND deleted_at = 0;
        SELECT id INTO err_log_menu_id FROM admin_menu WHERE route_name = 'ops_error-log' AND deleted_at = 0;
        SELECT id INTO task_menu_id FROM admin_menu WHERE route_name = 'ops_task' AND deleted_at = 0;
        SELECT id INTO config_menu_id FROM admin_menu WHERE route_name = 'manage_system_setting' AND deleted_at = 0;
        SELECT id INTO dict_menu_id FROM admin_menu WHERE route_name = 'manage_dict' AND deleted_at = 0;
        SELECT id INTO ipac_menu_id FROM admin_menu WHERE route_name = 'open-platform_ip-access' AND deleted_at = 0;
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

        -- 管理员管理API
        IF admin_menu_id IS NOT NULL THEN
            INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
            VALUES 
            (admin_menu_id, '获取管理员列表', 'GET', '/admin/v1/admins', '获取管理员列表', '1', NOW(), NOW()),
            (admin_menu_id, '获取管理员详情', 'GET', '/admin/v1/admins/:id', '获取管理员详情', '1', NOW(), NOW()),
            (admin_menu_id, '创建管理员', 'POST', '/admin/v1/admins', '创建管理员', '1', NOW(), NOW()),
            (admin_menu_id, '更新管理员', 'PUT', '/admin/v1/admins/:id', '更新管理员', '1', NOW(), NOW()),
            (admin_menu_id, '删除管理员', 'DELETE', '/admin/v1/admins/:id', '删除管理员', '1', NOW(), NOW()),
            (admin_menu_id, '批量删除管理员', 'DELETE', '/admin/v1/systemManage/deleteUsers', '批量删除管理员', '1', NOW(), NOW())
            ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
        END IF;

        -- 终端用户管理API
        IF user_menu_id IS NOT NULL THEN
            INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
            VALUES 
            (user_menu_id, '获取用户列表', 'GET', '/admin/v1/systemManage/users', '获取终端用户列表', '1', NOW(), NOW()),
            (user_menu_id, '查找用户自动补全', 'GET', '/admin/v1/systemManage/users/autocomplete', '查找用户自动补全', '1', NOW(), NOW()),
            (user_menu_id, '创建用户', 'POST', '/admin/v1/systemManage/users', '创建终端用户', '1', NOW(), NOW()),
            (user_menu_id, '更新用户', 'PUT', '/admin/v1/systemManage/users/:id', '更新终端用户', '1', NOW(), NOW()),
            (user_menu_id, '更新用户状态', 'PATCH', '/admin/v1/systemManage/users/:id/status', '更新终端用户状态', '1', NOW(), NOW()),
            (user_menu_id, '解锁用户', 'POST', '/admin/v1/systemManage/users/:id/unlock', '解锁登录锁定的终端用户', '1', NOW(), NOW()),
            (user_menu_id, '删除用户', 'DELETE', '/admin/v1/systemManage/users/:id', '删除终端用户', '1', NOW(), NOW())
            ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
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
            (role_menu_id, '更新角色菜单', 'PUT', '/admin/v1/systemManage/role/:id/menus', '更新角色菜单', '1', NOW(), NOW()),
            (role_menu_id, '批量删除角色', 'DELETE', '/admin/v1/systemManage/deleteRoles', '批量删除角色', '1', NOW(), NOW())
            ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
        END IF;

        -- 菜单管理API
        IF menu_menu_id IS NOT NULL THEN
            INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
            VALUES 
            (menu_menu_id, '获取菜单树', 'GET', '/admin/v1/systemManage/getMenuTree', '获取菜单树', '1', NOW(), NOW()),
            (menu_menu_id, '获取按钮权限', 'GET', '/admin/v1/systemManage/getButtonTree', '获取角色授权时的按钮权限', '1', NOW(), NOW()),
            (menu_menu_id, '获取API权限', 'GET', '/admin/v1/systemManage/getApiTree', '获取角色授权时的API权限', '1', NOW(), NOW()),
            (menu_menu_id, '添加菜单', 'POST', '/admin/v1/systemManage/addMenu', '添加菜单', '1', NOW(), NOW()),
            (menu_menu_id, '更新菜单', 'PUT', '/admin/v1/systemManage/updateMenu', '更新菜单', '1', NOW(), NOW()),
            (menu_menu_id, '删除菜单', 'DELETE', '/admin/v1/systemManage/deleteMenu', '删除菜单', '1', NOW(), NOW()),
            (menu_menu_id, '获取菜单列表', 'GET', '/admin/v1/systemManage/getMenuList', '获取菜单列表', '1', NOW(), NOW()),
            (menu_menu_id, '获取所有页面', 'GET', '/admin/v1/systemManage/getAllPages', '获取所有页面', '1', NOW(), NOW()),
            (menu_menu_id, '获取菜单详情', 'GET', '/admin/v1/systemManage/getMenu/:id', '获取菜单详情', '1', NOW(), NOW()),
            (menu_menu_id, '批量删除菜单', 'DELETE', '/admin/v1/systemManage/deleteMenus', '批量删除菜单', '1', NOW(), NOW())
            ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
        END IF;

        -- 按钮管理API
        IF button_menu_id IS NOT NULL THEN
            INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
            VALUES 
            (button_menu_id, '获取按钮列表', 'GET', '/admin/v1/systemManage/getButtonList', '获取按钮列表', '1', NOW(), NOW()),
            (button_menu_id, '添加按钮', 'POST', '/admin/v1/systemManage/addButton', '添加按钮', '1', NOW(), NOW()),
            (button_menu_id, '更新按钮', 'PUT', '/admin/v1/systemManage/updateButton', '更新按钮', '1', NOW(), NOW()),
            (button_menu_id, '删除按钮', 'DELETE', '/admin/v1/systemManage/deleteButton', '删除按钮', '1', NOW(), NOW())
            ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
        END IF;

        -- API管理API
        IF api_menu_id IS NOT NULL THEN
            INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
            VALUES 
            (api_menu_id, '获取API列表', 'GET', '/admin/v1/systemManage/getApiList', '获取API列表', '1', NOW(), NOW()),
            (api_menu_id, '添加API', 'POST', '/admin/v1/systemManage/addApi', '添加API', '1', NOW(), NOW()),
            (api_menu_id, '更新API', 'PUT', '/admin/v1/systemManage/updateApi', '更新API', '1', NOW(), NOW()),
            (api_menu_id, '删除API', 'DELETE', '/admin/v1/systemManage/deleteApi', '删除API', '1', NOW(), NOW())
            ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
        END IF;

    -- 运维日志API
    IF op_log_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (op_log_menu_id, '获取操作日志列表', 'GET', '/admin/v1/operation-logs', '获取操作日志列表', '1', NOW(), NOW()),
        (op_log_menu_id, '删除操作日志', 'DELETE', '/admin/v1/operation-logs/:id', '删除操作日志', '1', NOW(), NOW()),
        (op_log_menu_id, '批量删除操作日志', 'POST', '/admin/v1/operation-logs/batch-delete', '批量删除操作日志', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 任务调度API
    IF task_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (task_menu_id, '获取任务列表', 'GET', '/admin/v1/system/tasks', '获取后端任务列表', '1', NOW(), NOW()),
        (task_menu_id, '执行后台任务', 'POST', '/admin/v1/system/tasks/:name/run', '手动触发任务执行', '1', NOW(), NOW()),
        (task_menu_id, '启动后台任务', 'POST', '/admin/v1/system/tasks/:name/start', '启动指定的后台调度任务', '1', NOW(), NOW()),
        (task_menu_id, '停止后台任务', 'POST', '/admin/v1/system/tasks/:name/stop', '停止运行中的后台调度任务', '1', NOW(), NOW()),
        (task_menu_id, '重启后台任务', 'POST', '/admin/v1/system/tasks/:name/reload', '重新加载并启动任务', '1', NOW(), NOW()),
        (task_menu_id, '修改任务配置', 'PUT', '/admin/v1/system/tasks/update', '修改任务执行周期或启用状态', '1', NOW(), NOW()),
        (task_menu_id, '查询任务日志', 'GET', '/admin/v1/system/tasks/logs', '查询后台任务执行历史', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF err_log_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (err_log_menu_id, '获取错误日志列表', 'GET', '/admin/v1/error-logs', '获取错误日志列表', '1', NOW(), NOW()),
        (err_log_menu_id, '解决错误日志', 'PUT', '/admin/v1/error-logs/:id/resolve', '解决错误日志', '1', NOW(), NOW()),
        (err_log_menu_id, '删除错误日志', 'DELETE', '/admin/v1/error-logs/:id', '删除错误日志', '1', NOW(), NOW()),
        (err_log_menu_id, '批量删除错误日志', 'POST', '/admin/v1/error-logs/batch-delete', '批量删除错误日志', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 系统配置API
    IF config_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (config_menu_id, '查询系统配置列表', 'GET', '/admin/v1/system/configs', '查询系统配置列表', '1', NOW(), NOW()),
        (config_menu_id, '修改系统配置及热更新', 'PUT', '/admin/v1/system/configs', '修改系统配置及热更新', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 字典管理API
    IF dict_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (dict_menu_id, '获取字典类型列表', 'GET', '/admin/v1/system/dict/types', '获取字典类型管理列表', '1', NOW(), NOW()),
        (dict_menu_id, '创建字典类型', 'POST', '/admin/v1/system/dict/types', '创建新的字典类型', '1', NOW(), NOW()),
        (dict_menu_id, '更新字典类型', 'PUT', '/admin/v1/system/dict/types', '更新现有字典类型', '1', NOW(), NOW()),
        (dict_menu_id, '删除字典类型', 'DELETE', '/admin/v1/system/dict/types/:id', '删除字典类型', '1', NOW(), NOW()),
        (dict_menu_id, '获取字典数据列表', 'GET', '/admin/v1/system/dict/data', '获取字典数据管理列表', '1', NOW(), NOW()),
        (dict_menu_id, '获取字典数据详情', 'GET', '/admin/v1/system/dict/data/:code', '前端带缓存获取字典数据详情', '1', NOW(), NOW()),
        (dict_menu_id, '创建字典数据', 'POST', '/admin/v1/system/dict/data', '创建新的字典数据', '1', NOW(), NOW()),
        (dict_menu_id, '更新字典数据', 'PUT', '/admin/v1/system/dict/data', '更新现有字典数据', '1', NOW(), NOW()),
        (dict_menu_id, '删除字典数据', 'DELETE', '/admin/v1/system/dict/data/:id', '删除字典数据', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- IP 访问控制 API 初始化
    IF ipac_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (ipac_menu_id, '获取 IP 规则列表', 'GET', '/admin/v1/open-platform/ip-access', '获取 IP 规则列表', '1', NOW(), NOW()),
        (ipac_menu_id, '新增 IP 规则', 'POST', '/admin/v1/open-platform/ip-access', '新增 IP 规则', '1', NOW(), NOW()),
        (ipac_menu_id, '修改 IP 规则', 'PUT', '/admin/v1/open-platform/ip-access', '修改 IP 规则', '1', NOW(), NOW()),
        (ipac_menu_id, '删除 IP 规则', 'DELETE', '/admin/v1/open-platform/ip-access', '删除 IP 规则', '1', NOW(), NOW()),
        (ipac_menu_id, '批量删除规则', 'DELETE', '/admin/v1/open-platform/ip-access/batch', '批量删除规则', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 内容分类API
    IF category_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (category_menu_id, '获取分类列表', 'GET', '/admin/v1/content/categories', '获取分类列表', '1', NOW(), NOW()),
        (category_menu_id, '获取分类树', 'GET', '/admin/v1/content/categories/tree', '获取分类树', '1', NOW(), NOW()),
        (category_menu_id, '获取分类详情', 'GET', '/admin/v1/content/categories/:id', '获取分类详情', '1', NOW(), NOW()),
        (category_menu_id, '创建分类', 'POST', '/admin/v1/content/categories', '创建分类', '1', NOW(), NOW()),
        (category_menu_id, '更新分类', 'PUT', '/admin/v1/content/categories', '更新分类', '1', NOW(), NOW()),
        (category_menu_id, '删除分类', 'DELETE', '/admin/v1/content/categories/:id', '删除分类', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 内容文章API
    IF article_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (article_menu_id, '获取文章列表', 'GET', '/admin/v1/content/articles', '获取文章列表', '1', NOW(), NOW()),
        (article_menu_id, '获取文章详情', 'GET', '/admin/v1/content/articles/:id', '获取文章详情', '1', NOW(), NOW()),
        (article_menu_id, '创建文章', 'POST', '/admin/v1/content/articles', '创建文章', '1', NOW(), NOW()),
        (article_menu_id, '更新文章', 'PUT', '/admin/v1/content/articles', '更新文章', '1', NOW(), NOW()),
        (article_menu_id, '删除文章', 'DELETE', '/admin/v1/content/articles/:id', '删除文章', '1', NOW(), NOW()),
        (article_menu_id, '发布文章', 'PUT', '/admin/v1/content/articles/:id/publish', '发布文章', '1', NOW(), NOW()),
        (article_menu_id, '下架文章', 'PUT', '/admin/v1/content/articles/:id/unpublish', '下架文章', '1', NOW(), NOW()),
        (article_menu_id, '文章置顶', 'PUT', '/admin/v1/content/articles/:id/top', '文章置顶', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 横幅管理API
    IF banner_group_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (banner_group_menu_id, '获取Banner组列表', 'GET', '/admin/v1/content/banner-groups', '获取Banner组列表', '1', NOW(), NOW()),
        (banner_group_menu_id, '获取Banner组详情', 'GET', '/admin/v1/content/banner-groups/:id', '获取Banner组详情', '1', NOW(), NOW()),
        (banner_group_menu_id, '创建Banner组', 'POST', '/admin/v1/content/banner-groups', '创建Banner组', '1', NOW(), NOW()),
        (banner_group_menu_id, '更新Banner组', 'PUT', '/admin/v1/content/banner-groups', '更新Banner组', '1', NOW(), NOW()),
        (banner_group_menu_id, '删除Banner组', 'DELETE', '/admin/v1/content/banner-groups/:id', '删除Banner组', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF banner_item_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (banner_item_menu_id, '获取Banner项列表', 'GET', '/admin/v1/content/banner-items', '获取Banner项列表', '1', NOW(), NOW()),
        (banner_item_menu_id, '获取Banner项详情', 'GET', '/admin/v1/content/banner-items/:id', '获取Banner项详情', '1', NOW(), NOW()),
        (banner_item_menu_id, '创建Banner项', 'POST', '/admin/v1/content/banner-items', '创建Banner项', '1', NOW(), NOW()),
        (banner_item_menu_id, '更新Banner项', 'PUT', '/admin/v1/content/banner-items', '更新Banner项', '1', NOW(), NOW()),
        (banner_item_menu_id, '删除Banner项', 'DELETE', '/admin/v1/content/banner-items/:id', '删除Banner项', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 存储配置API
    IF storage_config_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (storage_config_menu_id, '获取存储配置列表', 'GET', '/admin/v1/storage-configs', '获取存储配置列表', '1', NOW(), NOW()),
        (storage_config_menu_id, '获取所有启用配置', 'GET', '/admin/v1/storage-configs/all-enabled', '获取所有启用配置', '1', NOW(), NOW()),
        (storage_config_menu_id, '获取存储配置详情', 'GET', '/admin/v1/storage-configs/:id', '获取存储配置详情', '1', NOW(), NOW()),
        (storage_config_menu_id, '创建存储配置', 'POST', '/admin/v1/storage-configs', '创建存储配置', '1', NOW(), NOW()),
        (storage_config_menu_id, '更新存储配置', 'PUT', '/admin/v1/storage-configs', '更新存储配置', '1', NOW(), NOW()),
        (storage_config_menu_id, '删除存储配置', 'DELETE', '/admin/v1/storage-configs/:id', '删除存储配置', '1', NOW(), NOW()),
        (storage_config_menu_id, '设置默认存储', 'PUT', '/admin/v1/storage-configs/:id/default', '设置默认存储', '1', NOW(), NOW()),
        (storage_config_menu_id, '测试存储上传', 'POST', '/admin/v1/storage-configs/test-upload', '测试存储上传', '1', NOW(), NOW()),
        (storage_config_menu_id, '获取上传凭证', 'POST', '/admin/v1/storage/upload-credentials', '获取上传凭证', '1', NOW(), NOW()),
        (storage_config_menu_id, '创建上传记录', 'POST', '/admin/v1/storage/upload-record', '创建上传记录', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 上传记录API
    IF upload_record_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (upload_record_menu_id, '获取上传记录列表', 'GET', '/admin/v1/upload-records', '获取上传记录列表', '1', NOW(), NOW()),
        (upload_record_menu_id, '获取上传记录详情', 'GET', '/admin/v1/upload-records/:id', '获取上传记录详情', '1', NOW(), NOW()),
        (upload_record_menu_id, '删除上传记录', 'DELETE', '/admin/v1/upload-records/:id', '删除上传记录', '1', NOW(), NOW()),
        (upload_record_menu_id, '批量删除上传记录', 'POST', '/admin/v1/upload-records/batch-delete', '批量删除上传记录', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 应用管理 API 初始化
    IF app_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (app_menu_id, '获取应用列表', 'GET', '/admin/v1/open/apps', '获取应用列表', '1', NOW(), NOW()),
        (app_menu_id, '新增应用', 'POST', '/admin/v1/open/apps', '新增应用', '1', NOW(), NOW()),
        (app_menu_id, '修改应用', 'PUT', '/admin/v1/open/apps', '修改应用', '1', NOW(), NOW()),
        (app_menu_id, '删除应用', 'DELETE', '/admin/v1/open/apps/:id', '删除应用', '1', NOW(), NOW()),
        (app_menu_id, '重置 AppSecret', 'PUT', '/admin/v1/open/apps/reset-secret', '重置 AppSecret', '1', NOW(), NOW()),
        (app_menu_id, '关联IP规则', 'PUT', '/admin/v1/open/apps/ip-rules', '关联IP规则到应用', '1', NOW(), NOW()),
        (app_menu_id, '获取应用权限', 'GET', '/admin/v1/open/apps/scopes', '获取应用的权限范围', '1', NOW(), NOW()),
        (app_menu_id, '获取可用权限', 'GET', '/admin/v1/open/apps/available-scopes', '获取所有可用的权限范围', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- API管理 API 初始化
    IF open_api_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (open_api_menu_id, '获取API列表', 'GET', '/admin/v1/open/apis', '获取API列表', '1', NOW(), NOW()),
        (open_api_menu_id, '新增API', 'POST', '/admin/v1/open/apis', '新增API', '1', NOW(), NOW()),
        (open_api_menu_id, '修改API', 'PUT', '/admin/v1/open/apis', '修改API', '1', NOW(), NOW()),
        (open_api_menu_id, '删除API', 'DELETE', '/admin/v1/open/apis/:id', '删除API', '1', NOW(), NOW()),
        (open_api_menu_id, '获取分组API', 'GET', '/admin/v1/open/apis/grouped', '获取按分组归类的API列表', '1', NOW(), NOW()),
        (open_api_menu_id, '获取权限关联API', 'GET', '/admin/v1/open/apis/scope-apis', '获取权限关联API', '1', NOW(), NOW()),
        (open_api_menu_id, '更新权限关联API', 'PUT', '/admin/v1/open/apis/scope-apis', '更新权限关联API', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 接口权限 API 初始化
    IF scope_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (scope_menu_id, '获取权限列表', 'GET', '/admin/v1/open/scopes', '获取权限列表', '1', NOW(), NOW()),
        (scope_menu_id, '新增权限', 'POST', '/admin/v1/open/scopes', '新增权限', '1', NOW(), NOW()),
        (scope_menu_id, '修改权限', 'PUT', '/admin/v1/open/scopes', '修改权限', '1', NOW(), NOW()),
        (scope_menu_id, '删除权限', 'DELETE', '/admin/v1/open/scopes/:id', '删除权限', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 开放平台日志 API 初始化
    IF open_log_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (open_log_menu_id, '获取调用日志列表', 'GET', '/admin/v1/ops/open-platform-log', '获取开放平台调用日志列表', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 消息中心 API
    IF msg_tpl_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (msg_tpl_menu_id, '获取模板列表', 'GET', '/admin/v1/message/templates', '获取消息模板列表', '1', NOW(), NOW()),
        (msg_tpl_menu_id, '新增模板', 'POST', '/admin/v1/message/templates', '新增消息模板', '1', NOW(), NOW()),
        (msg_tpl_menu_id, '更新模板', 'PUT', '/admin/v1/message/templates', '更新消息模板', '1', NOW(), NOW()),
        (msg_tpl_menu_id, '删除模板', 'DELETE', '/admin/v1/message/templates/:id', '删除消息模板', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF msg_sms_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES (msg_sms_menu_id, '发送短信', 'POST', '/admin/v1/message/send-sms', '发送短信通知', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF msg_email_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES (msg_email_menu_id, '发送邮件', 'POST', '/admin/v1/message/send-email', '发送邮件通知', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF msg_internal_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES (msg_internal_menu_id, '发送站内信', 'POST', '/admin/v1/message/send-internal', '发送系统站内信', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF msg_log_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (msg_log_menu_id, '获取发送记录', 'GET', '/admin/v1/message/records', '获取消息发送历史记录', '1', NOW(), NOW()),
        (msg_log_menu_id, '删除发送记录', 'DELETE', '/admin/v1/message/records/:id', '删除单条发送记录', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

END $$;

COMMIT;
