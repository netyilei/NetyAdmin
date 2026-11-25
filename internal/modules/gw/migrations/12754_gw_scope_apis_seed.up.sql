-- +goose Up
-- seed（11254 配对；scope→API 白名单全量映射——scope_id=组主键 ID
-- （gw-scope-<code>，与 12751 对齐；gw_scope_apis.scope_id 引用组主键）。
-- client 侧四组（C 端调用方 frontend-app 的授权面）；应用授权组即获组内全部 API 访问权；
INSERT INTO gw_scope_apis (scope_id, api_id) VALUES
('gw-scope-msg_inbox', 'gw-api-get-messages-inbox'),
('gw-scope-msg_inbox', 'gw-api-post-messages-inbox-read-all'),
('gw-scope-msg_inbox', 'gw-api-get-messages-inbox-unread-count'),
('gw-scope-msg_inbox', 'gw-api-post-messages-inbox-id-read'),
('gw-scope-storage_rw', 'gw-api-post-storage-complete'),
('gw-scope-storage_rw', 'gw-api-post-storage-credentials'),
('gw-scope-user_base', 'gw-api-post-users-login'),
('gw-scope-user_profile', 'gw-api-get-users-me'),
('gw-scope-user_profile', 'gw-api-put-users-phone'),
('gw-scope-user_base', 'gw-api-post-users-register'),
('gw-scope-user_profile', 'gw-api-put-users-username')
ON CONFLICT DO NOTHING;
-- +goose Down
SELECT 1; -- 同 12753：全量映射面 down 不清，按管理面精确操作
