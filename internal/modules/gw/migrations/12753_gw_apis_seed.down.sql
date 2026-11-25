-- +goose Down
-- 对称删除 12753 up 引入的 11 行（id 与 up 逐字一致——审查 P1-3 根修：旧列表
-- 为收缩前命名与 up 实际 ID 零匹配=回滚零命中留孤儿行）。
-- up 内 `-- +goose Down SELECT 1` 段为无效注释段（双文件模式下 goose 只解析
-- 本文件），其「登记面 down 不清」语义与 12751752 对称删除惯例冲突，以
-- 本文件为准；管理面先行创建的同 id 行（id 规则固定可复现）会被一并删除，
-- 属登记台账 seed 对称回滚的预期语义。
DELETE FROM gw_apis WHERE id IN (
    'gw-api-get-messages-inbox',
    'gw-api-post-messages-inbox-read-all',
    'gw-api-get-messages-inbox-unread-count',
    'gw-api-post-messages-inbox-id-read',
    'gw-api-post-storage-complete',
    'gw-api-post-storage-credentials',
    'gw-api-post-users-login',
    'gw-api-get-users-me',
    'gw-api-put-users-phone',
    'gw-api-post-users-register',
    'gw-api-put-users-username');
