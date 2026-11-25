-- +goose Up
-- gw_ip_rules：IP 访问控制规则（归组；判定=gw 引擎按应用启用组集严格合成——
-- 任一组 deny 命中即拒；allow 命中即放；组集空=不限 IP）。
CREATE TABLE gw_ip_rules (
    id         text PRIMARY KEY,              -- ULID
    group_id   text NOT NULL,                 -- 归属策略组
    cidrs      text NOT NULL,                 -- 逗号分隔 CIDR（引擎 parseCIDRs 口径）
    action     smallint NOT NULL,             -- 1=allow(白名单) 2=deny(黑名单)
    priority   int NOT NULL DEFAULT 0,        -- 组内展示排序
    enabled    bool NOT NULL DEFAULT true,
    remark     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_gw_ip_rules_group FOREIGN KEY (group_id) REFERENCES gw_ip_groups (id) ON DELETE CASCADE
);
-- +goose Down
DROP TABLE IF EXISTS gw_ip_rules;
