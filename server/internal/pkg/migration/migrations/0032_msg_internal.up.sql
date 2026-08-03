-- 站内信扩展表
CREATE TABLE IF NOT EXISTS msg_internal (
    id BIGSERIAL PRIMARY KEY,
    msg_record_id BIGINT NOT NULL,
    type SMALLINT DEFAULT 1, -- 1:系统公告, 2:私信
    CONSTRAINT fk_msg_internal_msg_record FOREIGN KEY (msg_record_id) REFERENCES msg_records(id) ON DELETE CASCADE
);

-- 高频查询: 所有站内信 JOIN 查询的核心关联列
CREATE INDEX IF NOT EXISTS idx_msg_internal_record_id ON msg_internal(msg_record_id);

