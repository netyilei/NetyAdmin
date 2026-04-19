BEGIN;

-- 消息模板种子数据
INSERT INTO msg_templates (code, name, channel, title, content, status) VALUES
('verify_code_sms', '短信验证码', 'sms', NULL, '您的验证码为：{{.code}}，有效期{{.expire}}分钟，请勿泄露。', 1),
('verify_code_email', '邮箱验证码', 'email', '验证码', '您的验证码为：{{.code}}，有效期{{.expire}}分钟，请勿泄露。', 1),
('welcome_internal', '欢迎注册', 'internal', '欢迎加入', '亲爱的{{.nickname}}，欢迎注册！', 1),
('reset_password_email', '重置密码', 'email', '重置密码通知', '您正在重置密码，验证码为：{{.code}}，有效期{{.expire}}分钟。如非本人操作请忽略。', 1),
('system_notice_internal', '系统公告', 'internal', '系统公告', '{{.content}}', 1)
ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;

COMMIT;
