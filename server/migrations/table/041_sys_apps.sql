BEGIN;

-- 应用信息表
CREATE TABLE IF NOT EXISTS sys_apps (
    id VARCHAR(26) PRIMARY KEY, -- ULID
    app_key VARCHAR(26) NOT NULL, -- 唯一标识 (同 ID)
    app_secret VARCHAR(255) NOT NULL, -- AES 加密存储的私钥
    name VARCHAR(100) NOT NULL, -- 应用名称
    status SMALLINT DEFAULT 1, -- 状态 (1: 启用, 0: 禁用)
    ip_strategy SMALLINT DEFAULT 1, -- IP 策略 (1: 黑名单模式, 2: 白名单模式)
    ip_filter_enabled BOOLEAN DEFAULT FALSE, -- 是否启用 IP 过滤
    quota_config JSONB, -- 限流配额配置 (QPS, Burst等)
    remark VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_apps_key ON sys_apps(app_key) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_apps_status ON sys_apps(status);

COMMIT;