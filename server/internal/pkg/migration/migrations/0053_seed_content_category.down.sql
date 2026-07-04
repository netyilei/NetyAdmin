-- 0053_seed_content_category.down.sql
-- 回滚 0053：删除种子初始化的内容分类（按 id 精确匹配）
DELETE FROM content_category
WHERE id IN (1, 2, 3, 4, 5, 6, 7, 8);
