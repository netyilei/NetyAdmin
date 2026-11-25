-- +goose Up
-- seed（11253 配对；网关 API 登记全量种子——2026-11-21 用户：登记台账=
-- 全部管理端 + client 面 API（从 OpenAPI spec 一次性提取，admin 92+client 11；
-- grp=业务模块）。ID 规则 gw-api-<method>-<path-slug> 固定可复现；后续新端点
-- 随手工增补或重跑提取脚本）。
INSERT INTO gw_apis (id, method, path, name, grp, description, status) VALUES
    ('gw-api-get-messages-inbox', 'GET', '/client/v1/messages/inbox', '站内信列表', 'messages', '', 1),
    ('gw-api-post-messages-inbox-read-all', 'POST', '/client/v1/messages/inbox/read-all', '站内信全部已读', 'messages', '', 1),
    ('gw-api-get-messages-inbox-unread-count', 'GET', '/client/v1/messages/inbox/unread-count', '站内信未读计数', 'messages', '', 1),
    ('gw-api-post-messages-inbox-id-read', 'POST', '/client/v1/messages/inbox/{id}/read', '站内信标记已读', 'messages', '', 1),
    ('gw-api-post-storage-complete', 'POST', '/client/v1/storage/complete', '完成上传（HMAC 校验 + CAS 登记）', 'storage', '', 1),
    ('gw-api-post-storage-credentials', 'POST', '/client/v1/storage/credentials', '签发直传凭证（presigned PUT + HMAC secret）', 'storage', '', 1),
    ('gw-api-post-users-login', 'POST', '/client/v1/users/login', '用户登录（User JWT）', 'users', '', 1),
    ('gw-api-get-users-me', 'GET', '/client/v1/users/me', '用户视图（PII 掩码示范）', 'users', '', 1),
    ('gw-api-put-users-phone', 'PUT', '/client/v1/users/phone', '修改手机号（登录后）', 'users', '', 1),
    ('gw-api-post-users-register', 'POST', '/client/v1/users/register', '用户注册（邮箱+密码——随机用户名）', 'users', '', 1),
    ('gw-api-put-users-username', 'PUT', '/client/v1/users/username', '修改用户名（登录后）', 'users', '', 1)
ON CONFLICT (id) DO NOTHING;
-- +goose Down
SELECT 1; -- 全量登记面——down 不清（运行时数据与登记同表，按 id 精确清理由管理面承担）
