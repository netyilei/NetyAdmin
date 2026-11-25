-- +goose Up
CREATE INDEX idx_rbac_menus_parent ON rbac_menus (parent_id);
