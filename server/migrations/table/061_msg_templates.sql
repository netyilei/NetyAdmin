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

COMMIT;