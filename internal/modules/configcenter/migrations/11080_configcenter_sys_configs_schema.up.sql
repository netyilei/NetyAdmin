-- +goose Up
-- sys_configs：动态配置键值（；架构 §10 分层第 4 层——默认值→文件→env→动态→Secrets）。
-- 键由管理面/部署者创建（无种子——运行时数据）；value_type 校验在 application 层。
CREATE TABLE sys_configs (
    id           text PRIMARY KEY,             -- ULID
    group_name   text NOT NULL,                -- 配置组（如 cors/log/features）
    config_key   text NOT NULL,                -- 组内键（如 whitelist）
    config_value text NOT NULL DEFAULT '',     -- 值（按 value_type 语义解析）
    value_type   smallint NOT NULL DEFAULT 1,  -- 1=string 2=int 3=bool 4=json
    description  text NOT NULL DEFAULT '',
    is_system    bool NOT NULL DEFAULT false,  -- 系统键：禁删/禁改 group+key
    is_public    bool NOT NULL DEFAULT false,  -- 公开键：经 /configs/public 免鉴权读
    updated_by   text NOT NULL DEFAULT 'system',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_sys_configs_group_key UNIQUE (group_name, config_key)
);
-- +goose Down
DROP TABLE IF EXISTS sys_configs;
