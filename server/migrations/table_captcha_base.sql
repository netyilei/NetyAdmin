BEGIN;

-- 1. 创建验证码存储表 (用于关闭缓存开关时)
CREATE TABLE IF NOT EXISTS "captcha_tokens" (
    "id" bigserial PRIMARY KEY,
    "captcha_id" varchar(50) UNIQUE NOT NULL,
    "answer" varchar(20) NOT NULL,
    "expire_at" timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS "idx_captcha_expire" ON "captcha_tokens" ("expire_at");

COMMIT;
