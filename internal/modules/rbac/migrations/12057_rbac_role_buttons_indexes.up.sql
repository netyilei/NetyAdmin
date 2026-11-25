-- +goose Up
CREATE INDEX idx_rbac_role_buttons_button ON rbac_role_buttons (button_id);
