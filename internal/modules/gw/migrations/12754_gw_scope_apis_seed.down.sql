-- +goose Down
-- 对称删除 12754 up 引入的映射（scope_id=组主键，与 12751 up 下划线命名逐字
-- 一致——审查 P1-3 根修：旧列表连字符命名与 up 零匹配=回滚零命中留悬空映射）。
DELETE FROM gw_scope_apis WHERE scope_id IN (
    'gw-scope-msg_inbox', 'gw-scope-storage_rw', 'gw-scope-user_base', 'gw-scope-user_profile');
