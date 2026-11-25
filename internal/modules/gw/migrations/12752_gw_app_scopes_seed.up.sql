-- +goose Up
-- seed（11252 配对；内置面应用 scope 授权种子——2026-11-21 用户终版：
-- **admin 端 API/WS 默认开放，授权方式=JWT+RBAC，不纳入应用限制面**——
-- admin-console 应用仅承载管理端 IP 组集/限流管控（ADR 0035），不做任何
-- scope 授权（admin API 的 admin_* 分组保留为「对第三方应用开放」的可选
-- 分组能力，接口权限页可授给任意开放应用）；frontend-app=C 端小程序调用方，
-- 授权 client 四组。app_id 引用 ensure_apps 固定 ID（本迁移先于 bootstrap
-- 应用创建执行，表无 FK，行先于应用就位）。
INSERT INTO gw_app_scopes (app_id, scope) VALUES
    ('gw-app-frontend-app', 'user_base'),
    ('gw-app-frontend-app', 'user_profile'),
    ('gw-app-frontend-app', 'msg_inbox'),
    ('gw-app-frontend-app', 'storage_rw')
ON CONFLICT DO NOTHING;
-- +goose Down
DELETE FROM gw_app_scopes WHERE app_id IN ('gw-app-admin-console', 'gw-app-frontend-app');
