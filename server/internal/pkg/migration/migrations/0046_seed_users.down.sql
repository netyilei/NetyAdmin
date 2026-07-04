-- 0046_seed_users.down.sql
-- 回滚 0046：删除种子初始化的测试用户（按 id 精确匹配）
DELETE FROM users
WHERE id IN (
    '01JQTESTUSER00001',
    '01JQTESTUSER00002',
    '01JQTESTUSER00003'
);
