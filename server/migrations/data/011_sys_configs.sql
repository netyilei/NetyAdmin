BEGIN;

INSERT INTO sys_configs (group_name, config_key, config_value, value_type, description, is_system, created_by, updated_by)
VALUES
('email_config', 'enabled', 'false', 'boolean', '邮件服务开关', FALSE, 1, 1),
('email_config', 'host', '', 'string', 'SMTP 服务器地址', FALSE, 1, 1),
('email_config', 'port', '465', 'number', 'SMTP 服务器端口', FALSE, 1, 1),
('email_config', 'user', '', 'string', '发件人账号', FALSE, 1, 1),
('email_config', 'password', '', 'string', '发件人密码/授权码', FALSE, 1, 1),
('email_config', 'from', '', 'string', '发件人地址', FALSE, 1, 1),
('sms_config', 'enabled', 'false', 'boolean', '短信服务开关', FALSE, 1, 1),
('sms_config', 'driver', 'tencent', 'string', '短信驱动 (tencent)', FALSE, 1, 1),
('sms_config', 'secret_id', '', 'string', 'SecretId', FALSE, 1, 1),
('sms_config', 'secret_key', '', 'string', 'SecretKey', FALSE, 1, 1),
('sms_config', 'app_id', '', 'string', 'AppId', FALSE, 1, 1),
('sms_config', 'sign_name', '', 'string', '短信签名', FALSE, 1, 1),
('msg_record_config', 'retention_days', '30', 'number', '发送记录保留天数', FALSE, 1, 1),
('captcha_config', 'admin_login_enabled', 'false', 'string', '管理员登录是否开启验证码', TRUE, 1, 1),
('captcha_config', 'user_register_enabled', 'false', 'string', '用户注册是否开启验证码', TRUE, 1, 1),
('captcha_config', 'user_login_pc', 'false', 'string', 'PC端用户登录是否开启验证码', TRUE, 1, 1),
('captcha_config', 'user_login_web', 'false', 'string', 'Web端用户登录是否开启验证码', TRUE, 1, 1),
('captcha_config', 'user_login_app', 'false', 'string', 'APP端用户登录是否开启验证码', TRUE, 1, 1),
('captcha_config', 'user_login_mobile', 'false', 'string', '移动端用户登录是否开启验证码', TRUE, 1, 1),
('captcha_config', 'captcha_length', '4', 'string', '验证码字符长度', TRUE, 1, 1),
('captcha_config', 'captcha_width', '240', 'string', '验证码图片宽度(px)', TRUE, 1, 1),
('captcha_config', 'captcha_height', '80', 'string', '验证码图片高度(px)', TRUE, 1, 1),
('captcha_config', 'captcha_type', 'digit', 'string', '验证码类型 (digit/string/math)', TRUE, 1, 1),
('captcha_config', 'captcha_expire', '600', 'string', '验证码过期时间(秒)', TRUE, 1, 1),
('cache_switches', 'captcha', 'true', 'string', '验证码缓存开关 (true:走缓存模块, false:走数据库)', TRUE, 1, 1)
ON CONFLICT (group_name, config_key) WHERE deleted_at = 0 DO UPDATE SET
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

COMMIT;