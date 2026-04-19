BEGIN;

INSERT INTO sys_open_apis (method, path, name, group_name, description, status, created_at, updated_at) VALUES
-- 用户认证（公开）
('POST', '/client/v1/user/register', '用户注册', '用户认证', 'C端用户注册', 1, NOW(), NOW()),
('POST', '/client/v1/user/login', '用户登录', '用户认证', 'C端用户登录', 1, NOW(), NOW()),
('POST', '/client/v1/user/refresh-token', '刷新令牌', '用户认证', '刷新访问令牌', 1, NOW(), NOW()),
('POST', '/client/v1/user/reset-password', '重置密码', '用户认证', '通过验证码重置密码', 1, NOW(), NOW()),
-- 用户资料（需签名）
('GET', '/client/v1/user/profile', '获取用户资料', '用户资料', '获取当前用户资料', 1, NOW(), NOW()),
('PUT', '/client/v1/user/profile', '修改用户资料', '用户资料', '修改当前用户资料', 1, NOW(), NOW()),
('PUT', '/client/v1/user/password', '修改密码', '用户资料', '修改当前用户密码', 1, NOW(), NOW()),
('DELETE', '/client/v1/user/account', '注销账户', '用户资料', '注销当前用户账户', 1, NOW(), NOW()),
('GET', '/client/v1/user/upload-token', '获取上传凭证', '用户资料', '获取存储上传凭证', 1, NOW(), NOW()),
('POST', '/client/v1/user/logout', '退出登录', '用户资料', '退出登录使令牌失效', 1, NOW(), NOW()),
-- 验证码（公开）
('GET', '/client/v1/auth/captcha', '获取验证码', '用户认证', '获取图形验证码', 1, NOW(), NOW()),
('GET', '/client/v1/auth/captcha-status', '验证码开关', '用户认证', '获取验证码启用状态', 1, NOW(), NOW()),
('GET', '/client/v1/auth/verify-config', '验证配置', '用户认证', '获取验证码发送配置', 1, NOW(), NOW()),
('POST', '/client/v1/auth/send-code', '发送验证码', '用户认证', '发送短信/邮件验证码', 1, NOW(), NOW())
ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

COMMIT;
