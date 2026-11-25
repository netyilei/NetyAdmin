-- +goose Up
-- 模块归属回填：可选模块的菜单（users/message/storage）；其余菜单保持 ''（常显）。
UPDATE rbac_menus SET module = 'users' WHERE id = 'rbac-menu-users';
UPDATE rbac_menus SET module = 'message' WHERE id IN ('rbac-menu-message', 'rbac-menu-smtp');
UPDATE rbac_menus SET module = 'storage' WHERE id = 'rbac-menu-storage';
