-- +goose Up
-- operation_logs：审计统一记录模型（架构 §8.5；AGENTS §11 admin 操作日志入 DB）。
-- 四类记录共用（type 区分）：operation/access/security/system；运行/错误日志禁入此表。
CREATE TABLE operation_logs (
    id          text PRIMARY KEY,               -- ULID
    type        smallint NOT NULL,              -- 1=operation 2=access 3=security 4=system
    actor_type  text NOT NULL,                  -- admin | user | service | app | system
    actor_id    text NOT NULL DEFAULT '',
    action      text NOT NULL,
    resource    text NOT NULL DEFAULT '',
    result      text NOT NULL DEFAULT 'success', -- success | deny | error
    biz_code    text NOT NULL DEFAULT '',        -- 5 位业务码
    ip          text NOT NULL DEFAULT '',
    user_agent  text NOT NULL DEFAULT '',
    request_id  text NOT NULL DEFAULT '',
    trace_id    text NOT NULL DEFAULT '',
    node_id     text NOT NULL DEFAULT '',
    detail      jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS operation_logs;
