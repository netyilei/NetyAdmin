-- =============================================
-- Open Platform - Scope Groups Table (For dynamic & i18n scopes)
-- =============================================

BEGIN;

CREATE TABLE IF NOT EXISTS sys_app_scope_groups (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL, -- 权限标识 (如 user_base)
    name VARCHAR(100) NOT NULL, -- 显示名称 (降级方案)
    i18n_key VARCHAR(100) NOT NULL, -- 国际化 Key (如 page.openPlatform.scope.userBase)
    description TEXT,
    status SMALLINT DEFAULT 1, -- 1: 启用, 0: 禁用
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scope_group_code ON sys_app_scope_groups(code) WHERE deleted_at = 0;

-- 插入默认权限数据
INSERT INTO sys_app_scope_groups (code, name, i18n_key) VALUES
('user_base', '用户基础 (注册/登录)', 'page.openPlatform.scope.userBase'),
('user_profile', '用户资料 (修改/注销)', 'page.openPlatform.scope.userProfile'),
('msg_send', '消息发送 (SMS/Email)', 'page.openPlatform.scope.msgSend'),
('content_view', '内容查看', 'page.openPlatform.scope.contentView')
ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET i18n_key = EXCLUDED.i18n_key;

COMMIT;
