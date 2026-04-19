BEGIN;

-- 站内信扩展表
CREATE TABLE IF NOT EXISTS msg_internal (
    id BIGSERIAL PRIMARY KEY,
    msg_record_id BIGINT NOT NULL,
    type SMALLINT DEFAULT 1 -- 1:系统公告, 2:私信
);

COMMIT;