-- =============================================
-- System Configs Module - Seed Data
-- =============================================

BEGIN;

-- 同步序列
DO $$
DECLARE
    seq_name TEXT;
BEGIN
    SELECT pg_get_serial_sequence('sys_configs', 'id') INTO seq_name;
    IF seq_name IS NULL THEN
        SELECT quote_ident(relname) INTO seq_name FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE relkind = 'S' AND n.nspname = 'public' AND relname = 'sys_configs_id_seq';
    END IF;
    IF seq_name IS NOT NULL THEN
        EXECUTE format('SELECT setval(%L, COALESCE((SELECT MAX(id) FROM sys_configs), 1))', seq_name);
    END IF;
END $$;

-- 初始化基础系统缓存开关
INSERT INTO sys_configs (group_name, config_key, config_value, value_type, description, is_system, created_by, updated_by)
VALUES 
('cache_switches', 'rbac_auth', 'true', 'boolean', '权限认证缓存开关', TRUE, 1, 1),
('cache_switches', 'sys_config', 'true', 'boolean', '系统配置缓存开关', TRUE, 1, 1),
('cache_switches', 'rbac_menu', 'true', 'boolean', '菜单权限缓存开关', TRUE, 1, 1),
('cache_switches', 'admin', 'true', 'boolean', '管理员数据缓存开关', TRUE, 1, 1),
('cache_switches', 'dict', 'true', 'boolean', '字典数据缓存开关', TRUE, 1, 1),
('cache_switches', 'storage', 'true', 'boolean', '存储配置缓存开关', TRUE, 1, 1),
('cache_switches', 'err_log_cache', 'true', 'boolean', '错误日志缓存开关', TRUE, 1, 1),
('cache_switches', 'content_category_cache', 'true', 'boolean', '内容分类树缓存开关', TRUE, 1, 1),
('system_params', 'admin_max_login_fails', '5', 'number', '管理员最多连续登录失败次数', TRUE, 1, 1),
-- 任务配置
('task_config', 'log_enabled', 'true', 'boolean', '日志记录开关', TRUE, 1, 1),
('task_config', 'retention_days', '30', 'number', '日志保留天数', TRUE, 1, 1),
('task_config', 'task:article_publish:enabled', 'true', 'boolean', '文章自动发布任务开关', TRUE, 1, 1)
ON CONFLICT (group_name, config_key) DO UPDATE SET 
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

COMMIT;
