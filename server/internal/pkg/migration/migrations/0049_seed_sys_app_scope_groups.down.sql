-- 0049_seed_sys_app_scope_groups.down.sql
-- 回滚 0049：删除种子初始化的权限分组（按 code 精确匹配）
DELETE FROM sys_app_scope_groups
WHERE code IN (
    'user_base',
    'user_profile',
    'msg_send',
    'msg_read',
    'content_view',
    'storage_upload',
    'echo_test'
);
