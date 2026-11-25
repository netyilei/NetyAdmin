-- +goose Up
-- bootstrap admin seed（11001 配对）：首启管理员由 Go 侧生成（随机密码 bcrypt
-- 无法在 SQL 内完成）——cmd 迁移命令调 identity.EnsureBootstrapAdmin；
-- 本文件仅占位保持 schema/seed 配对纪律。

-- +goose Down
SELECT 1;
