BEGIN;

-- 文章测试数据
INSERT INTO content_article (id, category_id, title, title_color, cover_image, summary, content, content_type, author, source, keywords, tags, is_top, top_sort, is_hot, is_recommend, allow_comment, view_count, like_count, comment_count, publish_status, published_at, scheduled_at, created_at, updated_at, deleted_at)
VALUES
    (1, 5, 'NetyAdmin 正式发布 V1.0 版本', '#333333', '', '我们很高兴地宣布，NetyAdmin 通用型多平台交易策略执行管理系统 V1.0 版本正式发布。', '正文内容...', 'richtext', '技术团队', 'NetyAdmin', 'NetyAdmin,交易系统', '发布,交易', true, 1, true, true, true, 128, 32, 5, 'published', NOW(), NULL, NOW(), NOW(), 0),
    (2, 5, '公司年度技术峰会圆满落幕', '#333333', '', '2024年度技术峰会于本月15日圆满落幕，本次峰会汇聚了来自全球的技术专家。', '正文内容...', 'richtext', '市场部', '内部', '技术峰会,金融科技', '峰会,技术', false, 0, false, true, true, 56, 12, 2, 'published', NOW(), NULL, NOW(), NOW(), 0),
    (3, 7, '全球金融市场2024年度回顾与展望', '#1a73e8', '', '本文将全面回顾2024年全球金融市场的发展态势，并对2025年进行展望。', '正文内容...', 'richtext', '研究部', 'NetyAdmin研究院', '金融市场,年度报告,投资', '年度报告,市场分析', true, 2, true, true, true, 256, 64, 12, 'published', NOW(), NULL, NOW(), NOW(), 0),
    (4, 7, '量化交易策略发展趋势分析', '#333333', '', '随着人工智能技术的发展，量化交易策略正在经历深刻变革。', '正文内容...', 'richtext', '研究部', 'NetyAdmin研究院', '量化交易,策略,人工智能', '量化,AI,策略', false, 0, true, false, true, 89, 23, 4, 'published', NOW(), NULL, NOW(), NOW(), 0),
    (5, 3, '系统维护公告', '#e53935', '', '为提升系统性能，我们将于本周六凌晨进行系统维护升级。', '正文内容...', 'richtext', '运维团队', 'NetyAdmin', '维护,公告,升级', '公告,维护', false, 0, false, false, true, 34, 5, 1, 'published', NOW(), NULL, NOW(), NOW(), 0),
    (6, 3, '新增API接口说明', '#333333', '', '本次更新新增了多个API接口，方便用户进行系统集成。', '正文内容...', 'richtext', '技术团队', 'NetyAdmin', 'API,接口,更新', 'API,更新', false, 0, false, false, true, 45, 8, 0, 'published', NOW(), NULL, NOW(), NOW(), 0),
    (7, 4, '如何快速上手交易系统', '#333333', '', '本文将介绍如何快速上手使用 NetyAdmin 交易系统。', '正文内容...', 'plaintext', '客服团队', 'NetyAdmin', '帮助,教程,入门', '教程,帮助', false, 0, false, false, true, 167, 34, 8, 'published', NOW(), NULL, NOW(), NOW(), 0),
    (8, 4, '常见问题解答', '#333333', '', '整理了用户最常遇到的问题及其解答。', '正文内容...', 'plaintext', '客服团队', 'NetyAdmin', 'FAQ,问题,解答', 'FAQ,帮助', false, 0, false, false, true, 89, 15, 3, 'published', NOW(), NULL, NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

COMMIT;