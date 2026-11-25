-- +goose Up
-- message_send_records：发送记录（——流水台账；retry Job 驱动
-- pending/failed → sent；超 MaxRetries 终态 failed）。
CREATE TABLE message_send_records (
    id          text PRIMARY KEY,            -- ULID
    template_id text NOT NULL,
    channel     text NOT NULL CHECK (channel IN ('email','sms')),
    target      text NOT NULL,               -- 邮箱/手机号（接收方）
    subject     text NOT NULL DEFAULT '',
    body        text NOT NULL,
    driver      text NOT NULL DEFAULT '',    -- 发送驱动名（smtp/log/下游自定义）
    status      smallint NOT NULL DEFAULT 1, -- 1=pending 2=sent 3=failed
    retries     int NOT NULL DEFAULT 0,
    error       text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    sent_at     timestamptz
);
-- +goose Down
DROP TABLE IF EXISTS message_send_records;
