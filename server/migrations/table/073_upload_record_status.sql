BEGIN;

-- 上传记录表增加状态机字段，用于「凭证签发 -> 上传成功通知」闭环
-- status:    pending(待上传) / uploaded(已上传) / expired(已过期)
-- secret:    凭证签发时生成的 HMAC 签名，与 recordID 组合防止只传 ID 的伪造攻击
-- expires_at: 凭证过期时间，超期未通知的 pending 记录由定时任务标记为 expired
--
-- 注意：status 默认值设为 'uploaded'，使历史数据（旧逻辑直接 INSERT 的已完成记录）
-- 自动为已上传状态，无需回填。

ALTER TABLE upload_record ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'uploaded';
ALTER TABLE upload_record ADD COLUMN IF NOT EXISTS secret VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE upload_record ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_upload_record_status ON upload_record(status);

COMMIT;
