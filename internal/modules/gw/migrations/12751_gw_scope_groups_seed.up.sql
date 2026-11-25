-- +goose Up
-- seed（11251 配对；scope 组定义——2026-11-21 用户语义定稿：scope 组=
-- 各业务模块的 API 授权分组，应用授权组即获组内全部 API 访问权。
-- client 侧四组（C 端调用方）+ admin 侧八模块组（管理端 API 对第三方应用
-- 开放的可选分组；admin 端自身默认开放走 JWT+RBAC 不经此面——2026-11-21））。
INSERT INTO gw_scope_groups (id, code, name, description) VALUES
    ('gw-scope-msg_inbox', 'msg_inbox', '消息收件箱', '站内信/通知读取与已读'),
    ('gw-scope-storage_rw', 'storage_rw', '存储直传', '对象存储直传凭证与确认'),
    ('gw-scope-user_base', 'user_base', '用户基础', '用户注册/登录'),
    ('gw-scope-user_profile', 'user_profile', '用户资料', '资料查看/用户名/手机号修改')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
