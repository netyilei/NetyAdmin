-- +goose Up
ALTER TABLE bus_outbox
    ADD COLUMN attempts        INT         NOT NULL DEFAULT 0,
    ADD COLUMN last_error      TEXT        NOT NULL DEFAULT '',
    ADD COLUMN next_attempt_at TIMESTAMPTZ;

-- +goose Down
-- bus_outbox relay 列 down：回退行级重试面（11332 配对）。
ALTER TABLE bus_outbox
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS attempts;
