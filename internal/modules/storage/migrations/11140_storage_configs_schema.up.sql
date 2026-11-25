-- +goose Up
-- storage_configs：存储配置（；V1 config 收敛——S3 通用协议字段面；
-- secret 出域面=驱动注入，DB 落原文（与 gw app_secret 差异：存储 secret 为
-- 外部服务凭据非平台加密钥——部署面负责外部服务安全；若需加密随 ST 后续批）。
CREATE TABLE storage_configs (
    id            text PRIMARY KEY,              -- ULID
    name          text NOT NULL,
    provider      text NOT NULL CHECK (provider IN ('minio','s3')),
    endpoint      text NOT NULL,
    region        text NOT NULL DEFAULT '',
    bucket_lookup text NOT NULL DEFAULT '' CHECK (bucket_lookup IN ('', 'path', 'virtual-host')),
                                                  -- S3 寻址风格旋钮（COS 强制虚拟主机域名；默认 path=MinIO 兼容）
    bucket        text NOT NULL,
    access_key    text NOT NULL,
    secret_key    text NOT NULL,
    domain        text NOT NULL DEFAULT '',
    path_prefix   text NOT NULL DEFAULT '',
    is_default    bool NOT NULL DEFAULT false,
    status        smallint NOT NULL DEFAULT 1,   -- 1=enabled 2=disabled
    max_file_size bigint NOT NULL DEFAULT 0,     -- 0=不限
    allowed_types text NOT NULL DEFAULT '',      -- 逗号分隔 MIME；空=不限
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_storage_configs_name UNIQUE (name)
);
-- +goose Down
DROP TABLE IF EXISTS storage_configs;
