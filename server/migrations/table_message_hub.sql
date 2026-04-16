-- =============================================
-- Message Hub Module - Tables
-- =============================================

BEGIN;

-- 消息模板表
CREATE TABLE IF NOT EXISTS msg_templates (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL, -- 模板唯一编码
    name VARCHAR(100) NOT NULL,
    channel VARCHAR(20) NOT NULL, -- sms, email, internal, push
    title VARCHAR(200), -- 邮件/站内信标题
    content TEXT NOT NULL, -- 模板内容
    provider_tpl_id VARCHAR(100), -- 第三方模板ID
    status SMALLINT DEFAULT 1, -- 1:启用, 0:禁用
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_tpl_code ON msg_templates(code) WHERE deleted_at = 0;

-- 消息记录表
CREATE TABLE IF NOT EXISTS msg_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT, -- 接收用户ID (可选)
    channel VARCHAR(20) NOT NULL,
    receiver VARCHAR(100) NOT NULL, -- 手机号/邮箱/Token
    title VARCHAR(200),
    content TEXT NOT NULL,
    status SMALLINT DEFAULT 0, -- 0:等待, 1:成功, 2:失败
    error_msg TEXT,
    node_id VARCHAR(50),
    priority SMALLINT DEFAULT 2, -- 1:高, 2:中, 3:低
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_msg_rec_user ON msg_records(user_id);
CREATE INDEX IF NOT EXISTS idx_msg_rec_status ON msg_records(status);

-- 站内信扩展表
CREATE TABLE IF NOT EXISTS msg_internal (
    id BIGSERIAL PRIMARY KEY,
    msg_record_id BIGINT NOT NULL,
    type SMALLINT DEFAULT 1, -- 1:系统公告, 2:私信
    FOREIGN KEY (msg_record_id) REFERENCES msg_records(id) ON DELETE CASCADE
);

-- 站内信已读记录表
CREATE TABLE IF NOT EXISTS msg_internal_reads (
    id BIGSERIAL PRIMARY KEY,
    msg_internal_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    read_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (msg_internal_id) REFERENCES msg_internal(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_int_read_user ON msg_internal_reads(msg_internal_id, user_id);

-- 消息黑名单表
CREATE TABLE IF NOT EXISTS msg_blacklist (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    channel VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_black_user_channel ON msg_blacklist(user_id, channel);

COMMIT;
