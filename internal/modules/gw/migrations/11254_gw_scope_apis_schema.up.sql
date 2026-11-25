-- +goose Up
-- gw_scope_apis：scope→API 白名单（§23.7 Scope 域——app 授权 scope，scope 白名单 API）。
CREATE TABLE gw_scope_apis (
    scope_id text NOT NULL,                    -- gw_scope_groups.id
    api_id   text NOT NULL,                    -- gw_apis.id
    CONSTRAINT pk_gw_scope_apis PRIMARY KEY (scope_id, api_id)
);
-- +goose Down
DROP TABLE IF EXISTS gw_scope_apis;
