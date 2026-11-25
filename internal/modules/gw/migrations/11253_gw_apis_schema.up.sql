-- +goose Up
-- gw_apis：网关 API 判定登记（V1 sys_open_apis 对齐——method+path 唯一语义）。
CREATE TABLE gw_apis (
    id          text PRIMARY KEY,              -- ULID
    method      text NOT NULL,
    path        text NOT NULL,                 -- 网关路径（/gateway/v1/...）
    name        text NOT NULL,
    grp         text NOT NULL DEFAULT '',      -- 分组（列名 grp——group 保留字规避）
    description text NOT NULL DEFAULT '',
    status      smallint NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_gw_apis_method_path UNIQUE (method, path)
);
-- +goose Down
DROP TABLE IF EXISTS gw_apis;
