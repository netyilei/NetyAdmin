BEGIN;

-- 站内信已读记录表
CREATE TABLE IF NOT EXISTS msg_internal_reads (
    id BIGSERIAL PRIMARY KEY,
    msg_internal_id BIGINT NOT NULL,
    user_id VARCHAR(26) NOT NULL,
    read_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_int_read_user ON msg_internal_reads(msg_internal_id, user_id);

COMMIT;