-- 0048_seed_sys_app_scopes.down.sql
-- 回滚 0048：删除为默认应用分配的种子权限（按 app_id + scope 精确匹配）
DELETE FROM sys_app_scopes
WHERE app_id = '01JQDEFAULTAPP001'
  AND scope IN (
    'user_base',
    'user_profile',
    'msg_send',
    'msg_read',
    'content_view'
);
