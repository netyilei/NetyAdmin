-- +goose Down
-- 精确 ID 列表（对照 up 全量；禁前缀 LIKE）。
DELETE FROM rbac_menus WHERE id = 'rbac-menu-smtp';
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-smtp-get',
    'rbac-api-smtp-put',
    'rbac-api-smtp-reset',
    'rbac-api-smtp-test'
);
