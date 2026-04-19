BEGIN;

-- 测试数据：内容分类
INSERT INTO content_category (id, parent_id, name, code, icon, content_type, sort, status, remark, created_at, updated_at, deleted_at)
VALUES
    (1, 0, '公司动态', 'company-news', 'ic:outline-business', 'richtext', 1, '1', '公司相关新闻动态', NOW(), NOW(), 0),
    (2, 0, '行业资讯', 'industry-news', 'ic:outline-trending-up', 'richtext', 2, '1', '行业相关资讯信息', NOW(), NOW(), 0),
    (3, 0, '产品公告', 'product-notice', 'ic:outline-notifications', 'richtext', 3, '1', '产品更新公告', NOW(), NOW(), 0),
    (4, 0, '帮助中心', 'help-center', 'ic:outline-help', 'plaintext', 4, '1', '用户帮助文档', NOW(), NOW(), 0),
    (5, 1, '企业新闻', 'enterprise-news', 'ic:outline-apartment', 'richtext', 1, '1', '企业内部新闻', NOW(), NOW(), 0),
    (6, 1, '活动报道', 'activity-report', 'ic:outline-event', 'richtext', 2, '1', '公司活动报道', NOW(), NOW(), 0),
    (7, 2, '市场分析', 'market-analysis', 'ic:outline-analytics', 'richtext', 1, '1', '市场分析报告', NOW(), NOW(), 0),
    (8, 2, '政策解读', 'policy-interpretation', 'ic:outline-gavel', 'richtext', 2, '1', '政策法规解读', NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

COMMIT;