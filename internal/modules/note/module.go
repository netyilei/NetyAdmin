// Package note 是 note 模块的装配入口。
// 职责边界：只做依赖装配与注册（Init 里注册路由/Job/Hook/事件/迁移/配置），
// 禁止 IO 与业务逻辑；业务在 application，实现细节在 adapters。
// 空骨架示例：下游实现 kernel/module 的 Module 契约（Name/DependsOn/Init）后在
// cmd 装配登记；生成同类骨架用 node scripts/scaffold/new-module.mjs。
package note
