-- 0056_seed_content_banner_item.down.sql
-- 回滚 0056：删除种子初始化的 Banner 项（按 id 精确匹配）
DELETE FROM content_banner_item
WHERE id IN (1, 2, 3, 4, 5, 6, 7);
