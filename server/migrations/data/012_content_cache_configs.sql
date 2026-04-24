BEGIN;

INSERT INTO sys_configs (group_name, config_key, config_value, value_type, description, is_system, created_by, updated_by)
VALUES
('content_cache', 'banner_cache_ttl', '30', 'number', 'Banner组/Banner缓存时间(分钟)', FALSE, 1, 1),
('content_cache', 'category_cache_ttl', '60', 'number', '文章分类缓存时间(分钟)', FALSE, 1, 1),
('content_cache', 'article_cache_ttl', '30', 'number', '文章详情缓存时间(分钟)', FALSE, 1, 1)
ON CONFLICT (group_name, config_key) WHERE deleted_at = 0 DO UPDATE SET
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

INSERT INTO sys_configs (group_name, config_key, config_value, value_type, description, is_system, created_by, updated_by)
VALUES
('cache_switches', 'content_article_cache', 'true', 'boolean', '内容文章缓存开关', TRUE, 1, 1),
('cache_switches', 'content_banner_cache', 'true', 'boolean', '内容Banner缓存开关', TRUE, 1, 1)
ON CONFLICT (group_name, config_key) WHERE deleted_at = 0 DO UPDATE SET
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

COMMIT;
