-- +goose Up
-- rbac_role_apis：角色↔API 授权（Casbin p 策略数据源）。
CREATE TABLE rbac_role_apis (
    role_id text NOT NULL,
    api_id  text NOT NULL,
    CONSTRAINT pk_rbac_role_apis PRIMARY KEY (role_id, api_id)
);
