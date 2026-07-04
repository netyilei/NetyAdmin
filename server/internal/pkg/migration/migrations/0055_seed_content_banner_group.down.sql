-- 0055_seed_content_banner_group.down.sql
-- 回滚 0055：删除种子初始化的 Banner 组（按 id 精确匹配）
DELETE FROM content_banner_group
WHERE id IN (1, 2, 3);
