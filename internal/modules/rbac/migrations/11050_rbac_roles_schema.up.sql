-- +goose Up
-- rbac_roles：角色定义。
CREATE TABLE rbac_roles (
    id         text PRIMARY KEY,             -- ULID
    code       text NOT NULL,                -- 角色码（Casbin p.sub；如 super-admin）
    name       text NOT NULL,
    status     smallint NOT NULL DEFAULT 1,  -- 1=enabled 2=disabled
    remark     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_rbac_roles_code UNIQUE (code)
);
