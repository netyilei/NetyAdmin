-- Drop orphan table msg_blacklist
-- 该表由 migration 0034 创建，但全代码库无 entity / repository / CRUD 引用
-- （无 GORM 模型映射，无任何查询），属于未落地的预留功能。
-- 决策（Round 7）：确认不做消息黑名单功能，drop 表清除债务。
-- down 迁移保留原始表结构以便回滚。

DROP TABLE IF EXISTS msg_blacklist CASCADE;
