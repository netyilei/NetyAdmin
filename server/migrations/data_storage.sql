-- =============================================
-- Storage Module - Data
-- =============================================

-- 权限数据初始化：菜单
DO $$
DECLARE
    settings_menu_id BIGINT;
    ops_menu_id BIGINT;
BEGIN
    SELECT id INTO settings_menu_id FROM admin_menu WHERE route_name = 'settings';
    SELECT id INTO ops_menu_id FROM admin_menu WHERE route_name = 'ops';

    -- 存储配置菜单 (基础设置下)
    IF settings_menu_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, created_at, updated_at)
        SELECT settings_menu_id, '存储配置', 'settings_storage-config', '/settings/storage-config', 'view.settings_storage-config', 'ic:outline-cloud-queue', 1, false, '1', '2', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'settings_storage-config');
    END IF;

    -- 上传记录菜单 (运维管理下)
    IF ops_menu_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, created_at, updated_at)
        SELECT ops_menu_id, '上传记录', 'ops_upload-record', '/ops/upload-record', 'view.ops_upload-record', 'ic:outline-upload-file', 3, false, '1', '2', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'ops_upload-record');
    END IF;
END $$;

-- 权限数据初始化：API
DO $$
DECLARE
    config_menu_id BIGINT;
    record_menu_id BIGINT;
BEGIN
    SELECT id INTO config_menu_id FROM admin_menu WHERE route_name = 'settings_storage-config';
    SELECT id INTO record_menu_id FROM admin_menu WHERE route_name = 'ops_upload-record';

    -- 存储配置API
    IF config_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (config_menu_id, '获取存储配置列表', 'GET', '/admin/v1/storage-configs', '获取存储配置列表', '1', NOW(), NOW()),
        (config_menu_id, '获取所有启用配置', 'GET', '/admin/v1/storage-configs/all-enabled', '获取所有启用配置', '1', NOW(), NOW()),
        (config_menu_id, '获取存储配置详情', 'GET', '/admin/v1/storage-configs/:id', '获取存储配置详情', '1', NOW(), NOW()),
        (config_menu_id, '创建存储配置', 'POST', '/admin/v1/storage-configs', '创建存储配置', '1', NOW(), NOW()),
        (config_menu_id, '更新存储配置', 'PUT', '/admin/v1/storage-configs', '更新存储配置', '1', NOW(), NOW()),
        (config_menu_id, '删除存储配置', 'DELETE', '/admin/v1/storage-configs/:id', '删除存储配置', '1', NOW(), NOW()),
        (config_menu_id, '设置默认存储', 'PUT', '/admin/v1/storage-configs/:id/default', '设置默认存储', '1', NOW(), NOW()),
        (config_menu_id, '测试存储上传', 'POST', '/admin/v1/storage-configs/test-upload', '测试存储上传', '1', NOW(), NOW()),
        (config_menu_id, '获取上传凭证', 'POST', '/admin/v1/storage/upload-credentials', '获取上传凭证', '1', NOW(), NOW()),
        (config_menu_id, '创建上传记录', 'POST', '/admin/v1/storage/upload-record', '创建上传记录', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
    END IF;

    -- 上传记录API
    IF record_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (record_menu_id, '获取上传记录列表', 'GET', '/admin/v1/upload-records', '获取上传记录列表', '1', NOW(), NOW()),
        (record_menu_id, '获取上传记录详情', 'GET', '/admin/v1/upload-records/:id', '获取上传记录详情', '1', NOW(), NOW()),
        (record_menu_id, '删除上传记录', 'DELETE', '/admin/v1/upload-records/:id', '删除上传记录', '1', NOW(), NOW()),
        (record_menu_id, '批量删除上传记录', 'POST', '/admin/v1/upload-records/batch-delete', '批量删除上传记录', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
    END IF;
END $$;

-- 清理：移除旧版残留的“批量删除存储配置”API
DELETE FROM admin_role_apis
WHERE admin_api_id IN (
    SELECT id FROM admin_api WHERE method = 'POST' AND path = '/admin/v1/storage-configs/batch-delete'
);

DELETE FROM admin_api
WHERE method = 'POST' AND path = '/admin/v1/storage-configs/batch-delete';

-- 权限数据初始化：按钮
DO $$
DECLARE
    config_menu_id BIGINT;
    record_menu_id BIGINT;
BEGIN
    SELECT id INTO config_menu_id FROM admin_menu WHERE route_name = 'settings_storage-config';
    SELECT id INTO record_menu_id FROM admin_menu WHERE route_name = 'ops_upload-record';

    -- 存储配置按钮
    IF config_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (config_menu_id, 'storage:add', '新增配置', NOW(), NOW()),
        (config_menu_id, 'storage:edit', '编辑配置', NOW(), NOW()),
        (config_menu_id, 'storage:delete', '删除配置', NOW(), NOW()),
        (config_menu_id, 'storage:test', '测试配置', NOW(), NOW()),
        (config_menu_id, 'storage:default', '设为默认', NOW(), NOW())
        ON CONFLICT (code) DO NOTHING;
    END IF;

    -- 上传记录按钮
    IF record_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (record_menu_id, 'ops:upload-record:delete', '删除', NOW(), NOW()),
        (record_menu_id, 'ops:upload-record:batch-delete', '批量删除', NOW(), NOW())
        ON CONFLICT (code) DO NOTHING;
    END IF;
END $$;

-- 自动授权：为超级管理员分配存储模块权限
DO $$
DECLARE
    super_role_id BIGINT;
BEGIN
    SELECT id INTO super_role_id FROM admin_role WHERE code = 'R_SUPER';

    IF super_role_id IS NOT NULL THEN
        -- 分配菜单 (通过 route_name 匹配)
        INSERT INTO admin_role_menus (admin_role_id, admin_menu_id)
        SELECT super_role_id, id FROM admin_menu 
        WHERE route_name IN ('settings_storage-config', 'ops_upload-record')
        ON CONFLICT DO NOTHING;

        -- 分配API (通过路径匹配)
        INSERT INTO admin_role_apis (admin_role_id, admin_api_id)
        SELECT super_role_id, id FROM admin_api 
        WHERE path LIKE '/admin/v1/storage%' OR path LIKE '/admin/v1/upload-records%'
        ON CONFLICT DO NOTHING;

        -- 分配按钮 (通过前缀匹配)
        INSERT INTO admin_role_buttons (admin_role_id, admin_button_id)
        SELECT super_role_id, id FROM admin_button 
        WHERE code LIKE 'storage:%' OR code LIKE 'ops:upload-record:%'
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- 重置主键序列
SELECT setval(pg_get_serial_sequence('storage_config', 'id'), (SELECT COALESCE(MAX(id), 1) FROM storage_config));
SELECT setval(pg_get_serial_sequence('upload_record', 'id'), (SELECT COALESCE(MAX(id), 1) FROM upload_record));
