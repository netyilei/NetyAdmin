-- +goose Up
-- admin_sessions：会话回退副本（session_store read_fallback: db 读源）。
CREATE TABLE admin_sessions (
    session_id text PRIMARY KEY,
    admin_id   text NOT NULL,
    payload    jsonb NOT NULL,
    expires_at timestamptz NOT NULL
);
