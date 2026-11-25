-- +goose Up
-- storage_upload_records：直传记录（；pending→completed，过期 pending
-- 由 cleanup Job 终态化 expired）。
CREATE TABLE storage_upload_records (
    id           text PRIMARY KEY,             -- ULID=record_id
    config_id    text NOT NULL,
    object_key   text NOT NULL,
    file_name    text NOT NULL DEFAULT '',
    file_size    bigint NOT NULL DEFAULT 0,    -- complete 时回填
    mime_type    text NOT NULL DEFAULT '',
    business_type text NOT NULL DEFAULT '',
    business_id  text NOT NULL DEFAULT '',
    secret       text NOT NULL,                -- HMAC 凭证（complete 校验）
    status       smallint NOT NULL DEFAULT 1,  -- 1=pending 2=completed 3=expired
    file_url     text NOT NULL DEFAULT '',
    uploader_id  text NOT NULL DEFAULT '',
    uploader_ip  text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
-- +goose Down
DROP TABLE IF EXISTS storage_upload_records;
