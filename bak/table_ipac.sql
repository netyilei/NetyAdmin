-- =============================================
-- IP Access Control Module - Tables
-- =============================================

BEGIN;

-- IP 访问控制表
CREATE TABLE IF NOT EXISTS sys_ip_access_control (
    id BIGSERIAL PRIMARY KEY,
    app_id VARCHAR(26), -- 所属应用 ID (ULID, 若为 NULL 则为全局规则)
    ip_addr VARCHAR(50) NOT NULL, -- IP 地址或 CIDR 网段
    type SMALLINT DEFAULT 2, -- 动作类型 (1: 放行/Allow, 2: 封禁/Deny)
    reason VARCHAR(255), -- 操作原因
    expired_at TIMESTAMP WITH TIME ZONE, -- 过期时间 (NULL 为永久)
    status SMALLINT DEFAULT 1, -- 状态 (1: 启用, 0: 禁用)
    created_by BIGINT, -- 操作人 ID
    updated_by BIGINT, -- 修改人 ID
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0 -- 软删除
);

-- 唯一索引: 同一个应用（或全局）下 IP 不能重复
CREATE UNIQUE INDEX IF NOT EXISTS idx_ipac_app_ip ON sys_ip_access_control(app_id, ip_addr) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_ipac_status ON sys_ip_access_control(status);
CREATE INDEX IF NOT EXISTS idx_ipac_deleted ON sys_ip_access_control(deleted_at);

COMMIT;
