-- +goose Up
-- gw_ip_groups：IP 访问控制策略组（ADR 0035——规则归组，应用引用组集；
-- 替代 ipac_rules.app_id 两层模型——「禁止>开放」组集严格合成归 gw 引擎）。
CREATE TABLE gw_ip_groups (
    id         text PRIMARY KEY,              -- ULID
    name       text NOT NULL,                 -- 唯一性由 12256 uk_gw_ip_groups_name 承载
    remark     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE IF EXISTS gw_ip_groups;
