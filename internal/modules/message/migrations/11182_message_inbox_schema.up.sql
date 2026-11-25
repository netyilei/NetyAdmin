-- +goose Up
-- message_inbox：站内信收件箱（；V1 client message 对齐——internal/read/
-- read-all/unread count；user_id 维度隔离——client authed 面经 users JWT）。
CREATE TABLE message_inbox (
    id         text PRIMARY KEY,             -- ULID
    user_id    text NOT NULL,                -- 接收用户（users 模块 ID）
    title      text NOT NULL DEFAULT '',
    body       text NOT NULL,
    is_read    bool NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    read_at    timestamptz
);
-- +goose Down
DROP TABLE IF EXISTS message_inbox;
