BEGIN;

INSERT INTO sys_app_scope_groups (code, name) VALUES
('user_base', '用户基础 (注册/登录)'),
('user_profile', '用户资料 (修改/注销)'),
('msg_send', '消息发送 (SMS/Email)'),
('msg_read', '站内信读取 (列表/详情/已读)'),
('content_view', '内容查看'),
('storage_upload', '存储上传 (凭证/记录)'),
('echo_test', '示例接口 (签名验证)')
ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;

COMMIT;