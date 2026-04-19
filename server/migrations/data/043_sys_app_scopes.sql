BEGIN;

-- 为默认应用分配全部权限
INSERT INTO sys_app_scopes (app_id, scope) VALUES
('01JQDEFAULTAPP001', 'user_base'),
('01JQDEFAULTAPP001', 'user_profile'),
('01JQDEFAULTAPP001', 'msg_send'),
('01JQDEFAULTAPP001', 'msg_read'),
('01JQDEFAULTAPP001', 'content_view')
ON CONFLICT (app_id, scope) DO NOTHING;

COMMIT;