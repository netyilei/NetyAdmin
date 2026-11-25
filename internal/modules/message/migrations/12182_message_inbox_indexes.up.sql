-- +goose Up
-- 收件箱查询面：未读计数（user+is_read 部分索引）/列表（user+时间倒序）。
CREATE INDEX idx_message_inbox_user_read ON message_inbox (user_id, is_read, created_at DESC);
CREATE INDEX idx_message_inbox_user_created ON message_inbox (user_id, created_at DESC);
-- +goose Down
DROP INDEX IF EXISTS idx_message_inbox_user_created;
DROP INDEX IF EXISTS idx_message_inbox_user_read;
