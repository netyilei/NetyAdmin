// Package module 提供 Module 契约、Manifest 解析校验、依赖 DAG 与 Registry 装配
// （可拆卸装配模型）。
//
// Module 接口（Name/DependsOn/Init）——Init 纯装配不做 IO；Manifest 与
// Module.DependsOn() 双源一致性在 Build 强制校验。
// Registry 八 registrar：Services（Expose/Resolve+ResolveAs 泛型，跨模块
// 调用）、Config（config_section 绑定， 闭环）、Hooks（转发 bootstrap.App）为
// 完整实现；Routes/Jobs/Events/Migrations/Security 为最小收集器（记录+去重，
// /// 各自替换强类型实现）。
// 可拆卸：edges 不 import 业务模块——业务路由自注册 RouteCollector，
// 跨模块调用走 Services.Resolve；装配清单单一入口在。
package module
