-- =============================================
-- Content Module - Data
-- =============================================

-- 权限数据初始化：菜单
DO $$
DECLARE
    content_root_id BIGINT;
BEGIN
    SELECT id INTO content_root_id FROM admin_menu WHERE route_name = 'content';

    -- 内容管理子菜单
    IF content_root_id IS NOT NULL THEN
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, created_at, updated_at)
        SELECT content_root_id, '内容分类', 'content_category', '/content/category', 'view.content_category', 'ic:outline-category', 1, false, '1', '2', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'content_category');

        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, created_at, updated_at)
        SELECT content_root_id, '文章管理', 'content_article', '/content/article', 'view.content_article', 'ic:outline-article', 2, false, '1', '2', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'content_article');

        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, created_at, updated_at)
        SELECT content_root_id, 'Banner组', 'content_banner-group', '/content/banner-group', 'view.content_banner-group', 'ic:outline-collections', 3, false, '1', '2', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'content_banner-group');

        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, created_at, updated_at)
        SELECT content_root_id, 'Banner管理', 'content_banner', '/content/banner', 'view.content_banner', 'ic:outline-image', 4, true, '1', '2', NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM admin_menu WHERE route_name = 'content_banner');
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
    SELECT id INTO category_menu_id FROM admin_menu WHERE route_name = 'content_category';
    SELECT id INTO article_menu_id FROM admin_menu WHERE route_name = 'content_article';
    SELECT id INTO banner_group_menu_id FROM admin_menu WHERE route_name = 'content_banner-group';
    SELECT id INTO banner_item_menu_id FROM admin_menu WHERE route_name = 'content_banner';

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
        ON CONFLICT (method, path) DO NOTHING;
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
        ON CONFLICT (method, path) DO NOTHING;
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
        ON CONFLICT (method, path) DO NOTHING;
    END IF;

    IF banner_item_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (banner_item_menu_id, '获取Banner项列表', 'GET', '/admin/v1/content/banner-items', '获取Banner项列表', '1', NOW(), NOW()),
        (banner_item_menu_id, '获取Banner项详情', 'GET', '/admin/v1/content/banner-items/:id', '获取Banner项详情', '1', NOW(), NOW()),
        (banner_item_menu_id, '创建Banner项', 'POST', '/admin/v1/content/banner-items', '创建Banner项', '1', NOW(), NOW()),
        (banner_item_menu_id, '更新Banner项', 'PUT', '/admin/v1/content/banner-items', '更新Banner项', '1', NOW(), NOW()),
        (banner_item_menu_id, '删除Banner项', 'DELETE', '/admin/v1/content/banner-items/:id', '删除Banner项', '1', NOW(), NOW())
        ON CONFLICT (method, path) DO NOTHING;
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
    SELECT id INTO category_menu_id FROM admin_menu WHERE route_name = 'content_category';
    SELECT id INTO article_menu_id FROM admin_menu WHERE route_name = 'content_article';
    SELECT id INTO banner_group_menu_id FROM admin_menu WHERE route_name = 'content_banner-group';
    SELECT id INTO banner_item_menu_id FROM admin_menu WHERE route_name = 'content_banner';

    -- 分类管理按钮
    IF category_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (category_menu_id, 'content:category:add', '添加分类', NOW(), NOW()),
        (category_menu_id, 'content:category:edit', '编辑分类', NOW(), NOW()),
        (category_menu_id, 'content:category:delete', '删除分类', NOW(), NOW())
        ON CONFLICT (code) DO NOTHING;
    END IF;

    -- 文章管理按钮
    IF article_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (article_menu_id, 'content:article:add', '发布文章', NOW(), NOW()),
        (article_menu_id, 'content:article:edit', '编辑文章', NOW(), NOW()),
        (article_menu_id, 'content:article:delete', '删除文章', NOW(), NOW()),
        (article_menu_id, 'content:article:publish', '上架/下架', NOW(), NOW()),
        (article_menu_id, 'content:article:top', '内置置顶', NOW(), NOW())
        ON CONFLICT (code) DO NOTHING;
    END IF;

    -- Banner管理按钮
    IF banner_group_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (banner_group_menu_id, 'content:banner-group:add', '新增分组', NOW(), NOW()),
        (banner_group_menu_id, 'content:banner-group:edit', '编辑分组', NOW(), NOW()),
        (banner_group_menu_id, 'content:banner-group:delete', '删除分组', NOW(), NOW())
        ON CONFLICT (code) DO NOTHING;
    END IF;

    IF banner_item_menu_id IS NOT NULL THEN
        INSERT INTO admin_button (menu_id, code, label, created_at, updated_at)
        VALUES 
        (banner_item_menu_id, 'content:banner:add', '添加项', NOW(), NOW()),
        (banner_item_menu_id, 'content:banner:edit', '编辑项', NOW(), NOW()),
        (banner_item_menu_id, 'content:banner:delete', '删除项', NOW(), NOW())
        ON CONFLICT (code) DO NOTHING;
    END IF;
END $$;

-- 自动授权：为超级管理员分配内容模块权限
DO $$
DECLARE
    super_role_id BIGINT;
BEGIN
    SELECT id INTO super_role_id FROM admin_role WHERE code = 'R_SUPER';

    IF super_role_id IS NOT NULL THEN
        -- 分配菜单
        INSERT INTO admin_role_menus (admin_role_id, admin_menu_id)
        SELECT super_role_id, id FROM admin_menu 
        WHERE route_name LIKE 'content_%'
        ON CONFLICT DO NOTHING;

        -- 分配API
        INSERT INTO admin_role_apis (admin_role_id, admin_api_id)
        SELECT super_role_id, id FROM admin_api 
        WHERE path LIKE '/admin/v1/content/%'
        ON CONFLICT DO NOTHING;

        -- 分配按钮
        INSERT INTO admin_role_buttons (admin_role_id, admin_button_id)
        SELECT super_role_id, id FROM admin_button 
        WHERE code LIKE 'content:%'
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- 测试数据：内容分类
INSERT INTO content_category (id, parent_id, name, code, icon, content_type, sort, status, remark, created_at, updated_at, deleted_at)
VALUES
    (1, 0, '公司动态', 'company-news', 'ic:outline-business', 'richtext', 1, '1', '公司相关新闻动态', NOW(), NOW(), 0),
    (2, 0, '行业资讯', 'industry-news', 'ic:outline-trending-up', 'richtext', 2, '1', '行业相关资讯信息', NOW(), NOW(), 0),
    (3, 0, '产品公告', 'product-notice', 'ic:outline-notifications', 'richtext', 3, '1', '产品更新公告', NOW(), NOW(), 0),
    (4, 0, '帮助中心', 'help-center', 'ic:outline-help', 'plaintext', 4, '1', '用户帮助文档', NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

-- 文章测试数据
INSERT INTO content_article (id, category_id, title, title_color, cover_image, summary, content, content_type, author, source, keywords, tags, is_top, top_sort, is_hot, is_recommend, allow_comment, view_count, like_count, comment_count, publish_status, published_at, scheduled_at, created_at, updated_at, deleted_at)
VALUES
    (1, 1, 'NetyAdmin 正式发布 V1.0 版本', '#333333', '', '我们很高兴地宣布，NetyAdmin 通用型多平台交易策略执行管理系统 V1.0 版本正式发布。', '正文内容...', 'richtext', '技术团队', 'NetyAdmin', 'NetyAdmin,交易系统', '发布,交易', true, 1, true, true, true, 128, 32, 5, 'published', NOW(), NULL, NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

-- 重置主键序列
SELECT setval(pg_get_serial_sequence('content_category', 'id'), (SELECT COALESCE(MAX(id), 1) FROM content_category));
SELECT setval(pg_get_serial_sequence('content_article', 'id'), (SELECT COALESCE(MAX(id), 1) FROM content_article));
SELECT setval(pg_get_serial_sequence('content_banner_group', 'id'), (SELECT COALESCE(MAX(id), 1) FROM content_banner_group));
SELECT setval(pg_get_serial_sequence('content_banner_item', 'id'), (SELECT COALESCE(MAX(id), 1) FROM content_banner_item));
