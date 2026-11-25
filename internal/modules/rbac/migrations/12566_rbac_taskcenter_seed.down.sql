-- +goose Down
-- 精确 ID 列表（对照 up 全量）——原 LIKE 'rbac-api-tc-%' 会在单独回滚 12566 时
-- 连带删除 12570 的 rbac-api-tc-dlq-replay（Standards J-8）；down 不受 checksum 约束
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-tc-jobs',
    'rbac-api-tc-runs',
    'rbac-api-tc-trigger',
    'rbac-api-tc-pause',
    'rbac-api-tc-resume'
);
