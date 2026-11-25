-- +goose Up
CREATE INDEX idx_rbac_buttons_menu ON rbac_buttons (menu_id);
