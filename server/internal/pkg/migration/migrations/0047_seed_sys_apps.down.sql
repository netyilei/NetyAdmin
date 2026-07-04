-- 0047_seed_sys_apps.down.sql
-- 回滚 0047：删除种子初始化的默认测试应用（按 id/app_key 精确匹配）
DELETE FROM sys_apps
WHERE id = '01JQDEFAULTAPP001';
