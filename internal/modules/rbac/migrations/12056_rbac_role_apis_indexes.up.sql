-- +goose Up
-- 反向查询：按 api 找授权角色（删除 API 前影响面确认）。
CREATE INDEX idx_rbac_role_apis_api ON rbac_role_apis (api_id);
