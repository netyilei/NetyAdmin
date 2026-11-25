-- +goose Up
--  统计面 API 权限码种子（G2：功能交付必须带权限数据；流水/快照全文件
-- 化 2026-11-19 ——查询端点读内存+快照文件，零 DB）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-gw-stats-overview', 'GET', '/admin/v1/open/stats/overview', 'admin.gw.stats.view', '网关调用总览'),
    ('rbac-api-gw-stats-trend', 'GET', '/admin/v1/open/stats/trend', 'admin.gw.stats.view', '网关调用趋势'),
    ('rbac-api-gw-stats-top', 'GET', '/admin/v1/open/stats/top', 'admin.gw.stats.view', '网关调用 Top 榜'),
    ('rbac-api-gw-stats-status', 'GET', '/admin/v1/open/stats/status', 'admin.gw.stats.view', '网关状态码分布'),
    ('rbac-api-gw-stats-latency', 'GET', '/admin/v1/open/stats/latency', 'admin.gw.stats.view', '网关延迟分位')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-gw-stats-overview', 'rbac-api-gw-stats-trend', 'rbac-api-gw-stats-top',
    'rbac-api-gw-stats-status', 'rbac-api-gw-stats-latency');
