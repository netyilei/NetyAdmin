-- 0050_seed_sys_open_apis.down.sql
-- 回滚 0050：删除种子初始化的开放平台 API（按 method + path 精确匹配）
DELETE FROM sys_open_apis
WHERE (method, path) IN (
    -- 用户认证（公开）
    ('POST', '/client/v1/user/register'),
    ('POST', '/client/v1/user/login'),
    ('POST', '/client/v1/user/refresh-token'),
    ('POST', '/client/v1/user/reset-password'),
    -- 用户资料（需签名）
    ('GET', '/client/v1/user/profile'),
    ('PUT', '/client/v1/user/profile'),
    ('PUT', '/client/v1/user/password'),
    ('DELETE', '/client/v1/user/account'),
    ('GET', '/client/v1/user/upload-token'),
    ('POST', '/client/v1/user/upload-record'),
    ('POST', '/client/v1/user/logout'),
    -- 验证码（公开）
    ('GET', '/client/v1/auth/captcha'),
    ('GET', '/client/v1/auth/captcha-status'),
    ('GET', '/client/v1/auth/verify-config'),
    ('POST', '/client/v1/auth/send-code'),
    -- 站内信（需签名）
    ('GET', '/client/v1/message/internal'),
    ('GET', '/client/v1/message/internal/:id'),
    ('PUT', '/client/v1/message/internal/read'),
    ('PUT', '/client/v1/message/internal/read-all'),
    ('GET', '/client/v1/message/internal/unread-count'),
    -- 内容管理（公开）
    ('GET', '/client/v1/content/categories/tree'),
    ('GET', '/client/v1/content/articles'),
    ('GET', '/client/v1/content/article/:id'),
    ('GET', '/client/v1/content/banners/:code'),
    -- 内容管理（需签名）
    ('POST', '/client/v1/content/article/:id/like'),
    ('POST', '/client/v1/content/banners/:id/click'),
    -- 示例接口（需签名）
    ('POST', '/client/v1/echo'),
    -- 存储上传（需签名）
    ('POST', '/client/v1/storage/credentials'),
    ('POST', '/client/v1/storage/records')
);
