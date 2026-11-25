-- +goose Up
CREATE INDEX idx_rbac_apis_status ON rbac_apis (status);
