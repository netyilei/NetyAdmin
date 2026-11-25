-- +goose Up
-- gw_scope_groups：scope 组定义（V1 sys_app_scope_groups 对齐——V2 无软删，
-- 状态位承载启停）。
CREATE TABLE gw_scope_groups (
    id          text PRIMARY KEY,              -- ULID
    code        text NOT NULL,                 -- scope code（如 user_base）
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    status      smallint NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_gw_scope_groups_code UNIQUE (code)
);
-- +goose Down
DROP TABLE IF EXISTS gw_scope_groups;
