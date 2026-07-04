-- 0054_seed_content_article.down.sql
-- 回滚 0054：删除种子初始化的内容文章（按 id 精确匹配）
DELETE FROM content_article
WHERE id IN (1, 2, 3, 4, 5, 6, 7, 8);
