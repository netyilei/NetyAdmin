-- +goose Up
-- message_templates：消息模板（；V1 message 模板对齐——占位符 {{key}}）。
CREATE TABLE message_templates (
    id         text PRIMARY KEY,             -- ULID
    name       text NOT NULL,
    channel    text NOT NULL CHECK (channel IN ('email','sms')), -- 通道
    subject    text NOT NULL DEFAULT '',     -- sms 通道为空
    body       text NOT NULL,                -- {{key}} 占位符
    status     smallint NOT NULL DEFAULT 1,  -- 1=enabled 2=disabled
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_message_templates_name UNIQUE (name)
);
-- +goose Down
DROP TABLE IF EXISTS message_templates;
