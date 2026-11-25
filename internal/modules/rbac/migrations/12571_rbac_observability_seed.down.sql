-- +goose Down
-- 精确 ID 列表（对照 up 全量；禁前缀 LIKE——避免连带删除其他 seed 行）。
DELETE FROM rbac_menus WHERE id = 'rbac-menu-observability';
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-obs-overview',
    'rbac-api-obs-metrics'
);
