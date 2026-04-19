-- =============================================
-- Content Module - Data
-- =============================================

BEGIN;

-- 同步序列：防止导入 dump 数据后主键序列落后导致的冲突
DO $$
DECLARE
    seq_name TEXT;
    table_names TEXT[] := ARRAY['content_category', 'content_article', 'content_banner_group', 'content_banner_item'];
    t_name TEXT;
BEGIN
    FOREACH t_name IN ARRAY table_names LOOP
        SELECT pg_get_serial_sequence(t_name, 'id') INTO seq_name;
        IF seq_name IS NULL THEN
            SELECT quote_ident(relname) INTO seq_name FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE relkind = 'S' AND n.nspname = 'public' AND relname = t_name || '_id_seq';
        END IF;
        IF seq_name IS NOT NULL THEN
            EXECUTE format('SELECT setval(%L, COALESCE((SELECT MAX(id) FROM %I), 1))', seq_name, t_name);
        END IF;
    END LOOP;
END $$;

-- 权限数据初始化：菜单
DO $$
DECLARE
    content_root_id BIGINT;
BEGIN
    SELECT id INTO content_root_id FROM admin_menu WHERE route_name = 'content';

    -- 内容管理子菜单
    IF content_root_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
        VALUES 
        (content_root_id, '内容分类', 'content_category', '/content/category', 'view.content_category', 'ic:outline-category', 1, false, '1', '2', 'route.content_category', NOW(), NOW()),
        (content_root_id, '文章管理', 'content_article', '/content/article', 'view.content_article', 'ic:outline-article', 2, false, '1', '2', 'route.content_article', NOW(), NOW()),
        (content_root_id, 'Banner组', 'content_banner-group', '/content/banner-group', 'view.content_banner-group', 'ic:outline-collections', 3, false, '1', '2', 'route.content_banner-group', NOW(), NOW()),
        (content_root_id, 'Banner管理', 'content_banner', '/content/banner', 'view.content_banner', 'ic:outline-image', 4, true, '1', '2', 'route.content_banner', NOW(), NOW())
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET i18_n_key = EXCLUDED.i18_n_key;
    END IF;
END $$;

-- 权限数据初始化：API
DO $$
DECLARE
    category_menu_id BIGINT;
    article_menu_id BIGINT;
    banner_group_menu_id BIGINT;
    banner_item_menu_id BIGINT;
BEGIN
    SELECT id INTO category_menu_id FROM admin_menu WHERE route_name = 'content_category' AND deleted_at = 0;
    SELECT id INTO article_menu_id FROM admin_menu WHERE route_name = 'content_article' AND deleted_at = 0;
    SELECT id INTO banner_group_menu_id FROM admin_menu WHERE route_name = 'content_banner-group' AND deleted_at = 0;
    SELECT id INTO banner_item_menu_id FROM admin_menu WHERE route_name = 'content_banner' AND deleted_at = 0;

    -- 内容分类API
    IF category_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (category_menu_id, '获取分类列表', 'GET', '/admin/v1/content/categories', '获取分类列表', '1', NOW(), NOW()),
        (category_menu_id, '获取分类树', 'GET', '/admin/v1/content/categories/tree', '获取分类树', '1', NOW(), NOW()),
        (category_menu_id, '获取分类详情', 'GET', '/admin/v1/content/categories/:id', '获取分类详情', '1', NOW(), NOW()),
        (category_menu_id, '创建分类', 'POST', '/admin/v1/content/categories', '创建分类', '1', NOW(), NOW()),
        (category_menu_id, '更新分类', 'PUT', '/admin/v1/content/categories', '更新分类', '1', NOW(), NOW()),
        (category_menu_id, '删除分类', 'DELETE', '/admin/v1/content/categories/:id', '删除分类', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 内容文章API
    IF article_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (article_menu_id, '获取文章列表', 'GET', '/admin/v1/content/articles', '获取文章列表', '1', NOW(), NOW()),
        (article_menu_id, '获取文章详情', 'GET', '/admin/v1/content/articles/:id', '获取文章详情', '1', NOW(), NOW()),
        (article_menu_id, '创建文章', 'POST', '/admin/v1/content/articles', '创建文章', '1', NOW(), NOW()),
        (article_menu_id, '更新文章', 'PUT', '/admin/v1/content/articles', '更新文章', '1', NOW(), NOW()),
        (article_menu_id, '删除文章', 'DELETE', '/admin/v1/content/articles/:id', '删除文章', '1', NOW(), NOW()),
        (article_menu_id, '发布文章', 'PUT', '/admin/v1/content/articles/:id/publish', '发布文章', '1', NOW(), NOW()),
        (article_menu_id, '下架文章', 'PUT', '/admin/v1/content/articles/:id/unpublish', '下架文章', '1', NOW(), NOW()),
        (article_menu_id, '文章置顶', 'PUT', '/admin/v1/content/articles/:id/top', '文章置顶', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 横幅管理API
    IF banner_group_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (banner_group_menu_id, '获取Banner组列表', 'GET', '/admin/v1/content/banner-groups', '获取Banner组列表', '1', NOW(), NOW()),
        (banner_group_menu_id, '获取Banner组详情', 'GET', '/admin/v1/content/banner-groups/:id', '获取Banner组详情', '1', NOW(), NOW()),
        (banner_group_menu_id, '创建Banner组', 'POST', '/admin/v1/content/banner-groups', '创建Banner组', '1', NOW(), NOW()),
        (banner_group_menu_id, '更新Banner组', 'PUT', '/admin/v1/content/banner-groups', '更新Banner组', '1', NOW(), NOW()),
        (banner_group_menu_id, '删除Banner组', 'DELETE', '/admin/v1/content/banner-groups/:id', '删除Banner组', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF banner_item_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (banner_item_menu_id, '获取Banner项列表', 'GET', '/admin/v1/content/banner-items', '获取Banner项列表', '1', NOW(), NOW()),
        (banner_item_menu_id, '获取Banner项详情', 'GET', '/admin/v1/content/banner-items/:id', '获取Banner项详情', '1', NOW(), NOW()),
        (banner_item_menu_id, '创建Banner项', 'POST', '/admin/v1/content/banner-items', '创建Banner项', '1', NOW(), NOW()),
        (banner_item_menu_id, '更新Banner项', 'PUT', '/admin/v1/content/banner-items', '更新Banner项', '1', NOW(), NOW()),
        (banner_item_menu_id, '删除Banner项', 'DELETE', '/admin/v1/content/banner-items/:id', '删除Banner项', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;

-- 权限数据初始化：按钮
DO $$
DECLARE
    category_menu_id BIGINT;
    article_menu_id BIGINT;
    banner_group_menu_id BIGINT;
    banner_item_menu_id BIGINT;
BEGIN
    SELECT id INTO category_menu_id FROM admin_menu WHERE route_name = 'content_category' AND deleted_at = 0;
    SELECT id INTO article_menu_id FROM admin_menu WHERE route_name = 'content_article' AND deleted_at = 0;
    SELECT id INTO banner_group_menu_id FROM admin_menu WHERE route_name = 'content_banner-group' AND deleted_at = 0;
    SELECT id INTO banner_item_menu_id FROM admin_menu WHERE route_name = 'content_banner' AND deleted_at = 0;

    -- 分类管理按钮
    IF category_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (category_menu_id, 'content:category:add', '新增分类', NOW(), NOW()),
        (category_menu_id, 'content:category:edit', '编辑分类', NOW(), NOW()),
        (category_menu_id, 'content:category:delete', '删除分类', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 文章管理按钮
    IF article_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (article_menu_id, 'content:article:add', '发布文章', NOW(), NOW()),
        (article_menu_id, 'content:article:edit', '修改文章', NOW(), NOW()),
        (article_menu_id, 'content:article:delete', '删除', NOW(), NOW()),
        (article_menu_id, 'content:article:publish', '上架', NOW(), NOW()),
        (article_menu_id, 'content:article:unpublish', '下架', NOW(), NOW()),
        (article_menu_id, 'content:article:top', '置顶', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- Banner管理按钮
    IF banner_group_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (banner_group_menu_id, 'content:banner-group:add', '新增', NOW(), NOW()),
        (banner_group_menu_id, 'content:banner-group:edit', '编辑', NOW(), NOW()),
        (banner_group_menu_id, 'content:banner-group:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    IF banner_item_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (banner_item_menu_id, 'content:banner:add', '新增', NOW(), NOW()),
        (banner_item_menu_id, 'content:banner:edit', '编辑', NOW(), NOW()),
        (banner_item_menu_id, 'content:banner:delete', '删除', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;

-- 自动授权：为超级管理员分配内容模块权限
DO $$
DECLARE
    super_role_id BIGINT;
BEGIN
    SELECT id INTO super_role_id FROM admin_role WHERE code = 'R_SUPER' AND deleted_at = 0;

    IF super_role_id IS NOT NULL THEN
        -- 分配菜单
        INSERT INTO admin_role_menus (admin_role_id, admin_menu_id)
        SELECT super_role_id, id FROM admin_menu 
        WHERE route_name LIKE 'content_%' AND deleted_at = 0
        ON CONFLICT (admin_role_id, admin_menu_id) DO NOTHING;

        -- 分配API
        INSERT INTO admin_role_apis (admin_role_id, admin_api_id)
        SELECT super_role_id, id FROM admin_api 
        WHERE path LIKE '/admin/v1/content/%' AND deleted_at = 0
        ON CONFLICT (admin_role_id, admin_api_id) DO NOTHING;

        -- 分配按钮
        INSERT INTO admin_role_buttons (admin_role_id, admin_button_id)
        SELECT super_role_id, id FROM admin_button 
        WHERE code LIKE 'content:%' AND deleted_at = 0
        ON CONFLICT (admin_role_id, admin_button_id) DO NOTHING;
    END IF;
END $$;

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

-- Banner组测试数据
INSERT INTO content_banner_group (id, name, code, description, position, width, height, sort, status, remark, created_at, updated_at, deleted_at)
VALUES
    (1, '首页轮播Banner', 'home-banner', '首页顶部轮播Banner', 'home-top', 1920, 500, 1, '1', '首页主要展示Banner', NOW(), NOW(), 0),
    (2, '登录页Banner', 'login-banner', '登录页面背景Banner', 'login-bg', 1920, 1080, 2, '1', '登录页背景展示', NOW(), NOW(), 0),
    (3, '侧边栏广告', 'sidebar-ad', '侧边栏广告位', 'sidebar', 300, 250, 3, '1', '侧边栏广告展示位', NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

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
