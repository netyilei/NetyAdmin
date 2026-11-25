-- +goose Up
-- 观测页权限与菜单种子（admin.observability.view；菜单挂运维目录）。
-- 权限码与 cmd/netyadmin/observability.go 常量同名（check-rbac-perms 对偶）；
-- component=前端视图路径 /src/views<component>.vue（admin/src/views/system/observability/index.vue）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-obs-overview', 'GET', '/admin/v1/system/observability/overview', 'admin.observability.view', '观测概览'),
    ('rbac-api-obs-metrics', 'GET', '/admin/v1/system/observability/metrics', 'admin.observability.view', '指标快照')
ON CONFLICT (id) DO NOTHING;

INSERT INTO rbac_menus (id, parent_id, name, path, component, icon, sort_order, hidden, status) VALUES
    ('rbac-menu-observability', 'rbac-menu-ops', '可观测', '/ops/observability', '/system/observability/index', 'DataLine', 35, false, 1)
ON CONFLICT (id) DO NOTHING;
