-- 0058_token_version.up.sql
-- BUG #5：为 users 和 admin_user 表新增 token_version 列，支持 Token 版本号机制
-- 用途：敏感操作（改密/禁用/删除/角色变更）递增版本号，使旧 token 立即失效，
--       作为 tokenStore 的纵深防御层（tokenStore 故障时仍能拒绝旧 token）。
-- 默认值 0；新部署首次启动后所有现有 token 仍有效（claims 中 TokenVersion 默认 0 与 DB 一致），
-- 直至该用户首次触发敏感操作。

ALTER TABLE users ADD COLUMN token_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE admin_user ADD COLUMN token_version BIGINT NOT NULL DEFAULT 0;
