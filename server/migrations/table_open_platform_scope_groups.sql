BEGIN;

CREATE TABLE IF NOT EXISTS sys_app_scope_groups (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    status SMALLINT DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scope_group_code ON sys_app_scope_groups(code) WHERE deleted_at = 0;

INSERT INTO sys_app_scope_groups (code, name) VALUES
('user_base', '用户基础 (注册/登录)'),
('user_profile', '用户资料 (修改/注销)'),
('msg_send', '消息发送 (SMS/Email)'),
('content_view', '内容查看')
ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;

COMMIT;
