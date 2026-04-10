-- =============================================
-- Storage Module - Data
-- =============================================

BEGIN;

-- 同步序列：防止导入 dump 数据后主键序列落后导致的冲突
DO $$
DECLARE
    seq_name TEXT;
    table_names TEXT[] := ARRAY['storage_config', 'upload_record'];
    t_name TEXT;
BEGIN
    FOREACH t_name IN ARRAY table_names LOOP
        SELECT pg_get_serial_sequence(t_name, 'id') INTO seq_name;
        IF seq_name IS NULL THEN
            SELECT quote_ident(relname) INTO seq_name FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE relkind = 'S' AND n.nspname = 'public' AND relname = t_name || '_id_seq';
        END IF;
        IF seq_name IS NOT NULL THEN
            EXECUTE format('SELECT setval(%L, COALESCE((SELECT MAX(id) FROM %I), 1))', seq_name, t_name);
        END IF;
    END LOOP;
END $$;

-- 权限数据初始化：菜单
DO $$
DECLARE
    settings_menu_id BIGINT;
    ops_menu_id BIGINT;
BEGIN
    SELECT id INTO settings_menu_id FROM admin_menu WHERE route_name = 'settings' AND deleted_at = 0;
    SELECT id INTO ops_menu_id FROM admin_menu WHERE route_name = 'ops' AND deleted_at = 0;

    -- 存储配置菜单 (基础设置下)
    IF settings_menu_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        VALUES 
        (settings_menu_id, '存储配置', 'settings_storage-config', '/settings/storage-config', 'view.settings_storage-config', 'ic:outline-cloud-queue', 1, false, '1', '2', 'route.settings_storage-config', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key;
    END IF;

    -- 上传记录菜单 (运维管理下)
    IF ops_menu_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        VALUES 
        (ops_menu_id, '上传记录', 'ops_upload-record', '/ops/upload-record', 'view.ops_upload-record', 'ic:outline-cloud-upload', 4, false, '1', '2', 'route.ops_upload-record', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key;
    END IF;
END $$;

-- 权限数据初始化：API
DO $$
DECLARE
    config_menu_id BIGINT;
    record_menu_id BIGINT;
BEGIN
    SELECT id INTO config_menu_id FROM admin_menu WHERE route_name = 'settings_storage-config' AND deleted_at = 0;
    SELECT id INTO record_menu_id FROM admin_menu WHERE route_name = 'ops_upload-record' AND deleted_at = 0;

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
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 上传记录API
    IF record_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (record_menu_id, '获取上传记录列表', 'GET', '/admin/v1/upload-records', '获取上传记录列表', '1', NOW(), NOW()),
        (record_menu_id, '获取上传记录详情', 'GET', '/admin/v1/upload-records/:id', '获取上传记录详情', '1', NOW(), NOW()),
        (record_menu_id, '删除上传记录', 'DELETE', '/admin/v1/upload-records/:id', '删除上传记录', '1', NOW(), NOW()),
        (record_menu_id, '批量删除上传记录', 'POST', '/admin/v1/upload-records/batch-delete', '批量删除上传记录', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;

-- 权限数据初始化：按钮
DO $$
DECLARE
    config_menu_id BIGINT;
    record_menu_id BIGINT;
BEGIN
    SELECT id INTO config_menu_id FROM admin_menu WHERE route_name = 'settings_storage-config' AND deleted_at = 0;
    SELECT id INTO record_menu_id FROM admin_menu WHERE route_name = 'ops_upload-record' AND deleted_at = 0;

    -- 存储配置按钮
    IF config_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (config_menu_id, 'storage:add', 'common.add', NOW(), NOW()),
        (config_menu_id, 'storage:edit', 'common.edit', NOW(), NOW()),
        (config_menu_id, 'storage:delete', 'common.delete', NOW(), NOW()),
        (config_menu_id, 'storage:test', 'page.settings.storageConfig.testUpload', NOW(), NOW()),
        (config_menu_id, 'storage:default', 'common.confirm', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 上传记录按钮
    IF record_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (record_menu_id, 'ops:upload-record:delete', 'common.delete', NOW(), NOW()),
        (record_menu_id, 'ops:upload-record:batch-delete', 'common.batchDelete', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;

-- 自动授权：为超级管理员分配存储模块权限
DO $$
DECLARE
    super_role_id BIGINT;
BEGIN
    SELECT id INTO super_role_id FROM admin_role WHERE code = 'R_SUPER' AND deleted_at = 0;

    IF super_role_id IS NOT NULL THEN
        -- 分配菜单 (通过 route_name 匹配)
        INSERT INTO admin_role_menus (admin_role_id, admin_menu_id)
        SELECT super_role_id, id FROM admin_menu 
        WHERE route_name IN ('settings_storage-config', 'ops_upload-record') AND deleted_at = 0
        ON CONFLICT (admin_role_id, admin_menu_id) DO NOTHING;

        -- 分配API (通过路径匹配)
        INSERT INTO admin_role_apis (admin_role_id, admin_api_id)
        SELECT super_role_id, id FROM admin_api 
        WHERE (path LIKE '/admin/v1/storage%' OR path LIKE '/admin/v1/upload-records%') AND deleted_at = 0
        ON CONFLICT (admin_role_id, admin_api_id) DO NOTHING;

        -- 分配按钮 (通过前缀匹配)
        INSERT INTO admin_role_buttons (admin_role_id, admin_button_id)
        SELECT super_role_id, id FROM admin_button 
        WHERE (code LIKE 'storage:%' OR code LIKE 'ops:upload-record:%') AND deleted_at = 0
        ON CONFLICT (admin_role_id, admin_button_id) DO NOTHING;
    END IF;
END $$;

COMMIT;
