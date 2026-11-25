-- +goose Up
-- admin_token_revocations：登出 jti 吊销的 DB 读源（read_fallback: db；
-- single 拓扑 Redis 宕机时生效）。expires_at=TTL 对齐 token 寿命（清理口径）。
CREATE TABLE admin_token_revocations (
    jti        text PRIMARY KEY,
    revoked_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
