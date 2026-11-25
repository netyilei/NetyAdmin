-- +goose Up
-- admins：管理员账户（ID-01；identity 收缩纯 admin——ADR 002130）。
CREATE TABLE admins (
    id                   text PRIMARY KEY,           -- ULID
    username             text NOT NULL,
    password_hash        text NOT NULL,              -- bcrypt（security.md §2）
    name                 text NOT NULL DEFAULT '',
    email                text NOT NULL DEFAULT '',   -- 可空；展示掩码归前端/ 出口
    phone                text NOT NULL DEFAULT '',   -- 手机号（PII——掩码归出口）
    avatar_url           text NOT NULL DEFAULT '',   -- 头像（data URL 或对象存储 URL—— 个人资料）
    status               smallint NOT NULL DEFAULT 1, -- 1=active 2=disabled
    must_change_password bool NOT NULL DEFAULT false,
    token_version        bigint NOT NULL DEFAULT 1,   -- 改密/封禁 ++ 全作废（tv）
    last_login_at        timestamptz,
    created_by           text NOT NULL DEFAULT 'system',
    updated_by           text NOT NULL DEFAULT 'system',
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_admins_username UNIQUE (username)
);
