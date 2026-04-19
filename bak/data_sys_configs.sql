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
('task_config', 'task:article_publish:enabled', 'true', 'boolean', '文章自动发布任务开关', TRUE, 1, 1),
-- 操作日志配置
('ops_config', 'retention_days', '30', 'number', '操作日志保留天数', TRUE, 1, 1),
-- 错误日志配置
('error_config', 'retention_days', '30', 'number', '错误日志保留天数', TRUE, 1, 1),
-- 用户与认证配置 (User Module)
('user_config', 'storage_module', '', 'string', '用户文件存储驱动 (留空使用默认)', FALSE, 1, 1),
('user_config', 'login_storage', 'memory', 'string', '用户登录态存储介质 (memory/db)', FALSE, 1, 1),
('user_config', 'token_expire', '86400', 'number', '用户 Token 过期时间 (秒)', FALSE, 1, 1),
('user_config', 'login_max_retry', '5', 'number', '用户登录最大重试次数', FALSE, 1, 1),
('user_config', 'login_lock_duration', '3600', 'number', '用户登录锁定时间 (秒)', FALSE, 1, 1),
('user_config', 'password_min_length', '6', 'number', '用户密码最小长度', FALSE, 1, 1),
('user_config', 'password_require_types', '2', 'number', '用户密码复杂度要求类型数 (1-4)', FALSE, 1, 1),
('user_config', 'user_register_verify', 'false', 'boolean', '用户注册是否需要验证', FALSE, 1, 1),
('user_config', 'user_register_verify_type', 'email', 'string', '用户注册验证方式 (email/sms)', FALSE, 1, 1),
('user_config', 'user_reset_pwd_verify', 'true', 'boolean', '用户找回密码是否需要验证', FALSE, 1, 1),
('user_config', 'user_reset_pwd_verify_type', 'email', 'string', '用户找回密码验证方式 (email/sms)', FALSE, 1, 1),
-- 验证码配置
('captcha_config', 'user_login_captcha_enabled', 'true', 'boolean', '用户登录是否启用验证码', FALSE, 1, 1),
('captcha_config', 'user_reset_pwd_captcha_enabled', 'true', 'boolean', '用户重置密码是否启用验证码', FALSE, 1, 1),
('captcha_config', 'captcha_type', 'digit', 'string', '验证码类型 (digit/string/math)', TRUE, 1, 1),
('captcha_config', 'captcha_length', '4', 'number', '验证码长度', TRUE, 1, 1),
('captcha_config', 'captcha_width', '120', 'number', '验证码宽度', TRUE, 1, 1),
('captcha_config', 'captcha_height', '40', 'number', '验证码高度', TRUE, 1, 1),
('captcha_config', 'captcha_expire', '300', 'number', '验证码过期时间 (秒)', TRUE, 1, 1)
ON CONFLICT (group_name, config_key) WHERE deleted_at = 0 DO UPDATE SET 
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

COMMIT;
