// Package database 提供连接管理、UnitOfWork（OnCommit/OnRollback）、模块迁移执行器与只读副本。
// 迁移：5 位号段、schema/seed 分离、checksum 拦截改动（docs/rules/migrations.md）。
// 红线：禁跨模块 JOIN/事务；禁 GORM AutoMigrate。
package database
