-- =============================================
-- Captcha Module - Seed Data
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

-- 2. 验证码基础业务配置 (captcha_config 分组)
INSERT INTO "sys_configs" ("group_name", "config_key", "config_value", "description", "is_system", "value_type")
VALUES 
('captcha_config', 'admin_login_enabled', 'false', '管理员登录是否开启验证码', TRUE, 'string'),
('captcha_config', 'user_register_enabled', 'false', '用户注册是否开启验证码', TRUE, 'string'),
('captcha_config', 'user_login_pc', 'false', 'PC端用户登录是否开启验证码', TRUE, 'string'),
('captcha_config', 'user_login_web', 'false', 'Web端用户登录是否开启验证码', TRUE, 'string'),
('captcha_config', 'user_login_app', 'false', 'APP端用户登录是否开启验证码', TRUE, 'string'),
('captcha_config', 'user_login_mobile', 'false', '移动端用户登录是否开启验证码', TRUE, 'string'),
('captcha_config', 'captcha_length', '4', '验证码字符长度', TRUE, 'string'),
('captcha_config', 'captcha_width', '240', '验证码图片宽度(px)', TRUE, 'string'),
('captcha_config', 'captcha_height', '80', '验证码图片高度(px)', TRUE, 'string'),
('captcha_config', 'captcha_type', 'digit', '验证码类型 (digit/string/math)', TRUE, 'string'),
('captcha_config', 'captcha_expire', '600', '验证码过期时间(秒)', TRUE, 'string')
ON CONFLICT ("group_name", "config_key") WHERE deleted_at = 0 DO UPDATE SET 
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

-- 3. 验证码缓存开关 (cache_switches 分组)
INSERT INTO "sys_configs" ("group_name", "config_key", "config_value", "description", "is_system", "value_type")
VALUES 
('cache_switches', 'captcha', 'true', '验证码缓存开关 (true:走缓存模块, false:走数据库)', TRUE, 'string')
ON CONFLICT ("group_name", "config_key") WHERE deleted_at = 0 DO UPDATE SET 
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

COMMIT;
