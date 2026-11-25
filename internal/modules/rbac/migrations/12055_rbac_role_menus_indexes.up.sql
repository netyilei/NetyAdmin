-- +goose Up
CREATE INDEX idx_rbac_role_menus_menu ON rbac_role_menus (menu_id);
