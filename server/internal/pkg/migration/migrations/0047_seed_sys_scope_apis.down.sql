-- 0051_seed_sys_scope_apis.down.sql
-- 回滚 0051：删除种子初始化的 scope-api 绑定（仅清理种子 scope group 的绑定）
-- 注意：up 文件按 scope group 动态查找 scope_id 绑定 API；down 文件沿用相同思路，
--       仅删除「种子 scope group 与种子 open API 的绑定关系」，不影响用户手动添加的其他绑定
DELETE FROM sys_scope_apis
WHERE scope_id IN (
    SELECT id FROM sys_app_scope_groups
    WHERE code IN ('user_base', 'user_profile', 'msg_send', 'msg_read', 'content_view', 'storage_upload', 'echo_test')
)
AND api_id IN (
    SELECT id FROM sys_open_apis
    WHERE path IN (
        '/client/v1/message/internal',
        '/client/v1/message/internal/:id',
        '/client/v1/message/internal/read',
        '/client/v1/message/internal/read-all',
        '/client/v1/message/internal/unread-count',
        '/client/v1/content/categories/tree',
        '/client/v1/content/articles',
        '/client/v1/content/article/:id',
        '/client/v1/content/banners/:code',
        '/client/v1/content/article/:id/like',
        '/client/v1/content/banners/:id/click',
        '/client/v1/user/register',
        '/client/v1/user/login',
        '/client/v1/user/refresh-token',
        '/client/v1/user/reset-password',
        '/client/v1/auth/captcha',
        '/client/v1/auth/captcha-status',
        '/client/v1/auth/verify-config',
        '/client/v1/user/profile',
        '/client/v1/user/password',
        '/client/v1/user/account',
        '/client/v1/user/upload-token',
        '/client/v1/user/upload-record',
        '/client/v1/user/logout',
        '/client/v1/auth/send-code',
        '/client/v1/storage/credentials',
        '/client/v1/storage/records',
        '/client/v1/echo'
    )
);
