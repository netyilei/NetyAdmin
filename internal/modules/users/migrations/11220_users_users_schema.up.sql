-- +goose Up
-- users：C 端用户（US-01 演示模块；ADR 0030——email/phone PII 掩码示范，
-- token_version= per-row 吊销机制面）。
CREATE TABLE users (
    id             text PRIMARY KEY,             -- ULID
    email          text NOT NULL,
    username       text NOT NULL,                -- 随机生成（注册）→ 登录后可改
    phone          text NOT NULL DEFAULT '',     -- PII（可空）
    password_hash  text NOT NULL,                -- bcrypt cost 12
    status         smallint NOT NULL DEFAULT 1 CHECK (status IN (1, 2)), -- 1=enabled 2=disabled（D2 域校验双闸）
    token_version  bigint NOT NULL DEFAULT 1,    --  TokenVersion
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_users_email UNIQUE (email),
    CONSTRAINT uk_users_username UNIQUE (username)
);
-- +goose Down
DROP TABLE IF EXISTS users;
