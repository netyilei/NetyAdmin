BEGIN;

-- Banner项测试数据
INSERT INTO content_banner_item (id, group_id, title, subtitle, image_url, link_type, link_url, sort, status, created_at, updated_at, deleted_at)
VALUES
    (1, 1, '正式发布', '全新架构，性能升级', 'https://picsum.photos/1920/500?random=1', 'none', '', 1, '1', NOW(), NOW(), 0),
    (2, 1, '专业交易解决方案', '多平台支持，安全可靠', 'https://picsum.photos/1920/500?random=2', 'none', '', 2, '1', NOW(), NOW(), 0),
    (3, 1, '高效策略执行', '毫秒级响应，稳定运行', 'https://picsum.photos/1920/500?random=3', 'none', '', 3, '1', NOW(), NOW(), 0),
    (4, 2, '安全登录', '您的数据安全是我们的首要任务', 'https://picsum.photos/1920/1080?random=4', 'none', '', 1, '1', NOW(), NOW(), 0),
    (5, 2, '专业服务', '7x24小时技术支持', 'https://picsum.photos/1920/1080?random=5', 'none', '', 2, '1', NOW(), NOW(), 0),
    (6, 3, '限时优惠活动', '新用户注册即享优惠', 'https://picsum.photos/300/250?random=6', 'none', '', 1, '1', NOW(), NOW(), 0),
    (7, 3, '技术文档', '查看详细开发文档', 'https://picsum.photos/300/250?random=7', 'none', '', 2, '1', NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

COMMIT;