-- +goose Down
UPDATE rbac_menus SET module = '' WHERE id IN (
    'rbac-menu-users',
    'rbac-menu-message',
    'rbac-menu-smtp',
    'rbac-menu-storage'
);
