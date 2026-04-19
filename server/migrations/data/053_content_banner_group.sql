BEGIN;

-- Banner组测试数据
INSERT INTO content_banner_group (id, name, code, description, position, width, height, sort, status, remark, created_at, updated_at, deleted_at)
VALUES
    (1, '首页轮播Banner', 'home-banner', '首页顶部轮播Banner', 'home-top', 1920, 500, 1, '1', '首页主要展示Banner', NOW(), NOW(), 0),
    (2, '登录页Banner', 'login-banner', '登录页面背景Banner', 'login-bg', 1920, 1080, 2, '1', '登录页背景展示', NOW(), NOW(), 0),
    (3, '侧边栏广告', 'sidebar-ad', '侧边栏广告位', 'sidebar', 300, 250, 3, '1', '侧边栏广告展示位', NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

COMMIT;