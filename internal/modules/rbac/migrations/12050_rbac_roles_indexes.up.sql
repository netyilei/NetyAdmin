-- +goose Up
CREATE INDEX idx_rbac_roles_status ON rbac_roles (status);
