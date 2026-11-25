-- +goose Up
-- 索引（查询面：records 列表按 channel/status 过滤+时间排序；retry 扫描；
-- 模板 name 查重已由 UNIQUE 承担）。
CREATE INDEX idx_message_records_channel_status ON message_send_records (channel, status, created_at DESC);
CREATE INDEX idx_message_records_created ON message_send_records (created_at DESC);
CREATE INDEX idx_message_records_template ON message_send_records (template_id);
-- +goose Down
DROP INDEX IF EXISTS idx_message_records_template;
DROP INDEX IF EXISTS idx_message_records_created;
DROP INDEX IF EXISTS idx_message_records_channel_status;
