// Package application 是 note 应用层：用例/应用服务、事务边界（UnitOfWork）、编排。
//
// 允许：import 本模块 domain、其他模块 api/ 端口与 DTO、kernel/errorx（错误映射用）。
// 禁止：import 任何 adapters、具体 Redis/GORM 客户端、HTTP 类型（CI check-imports 强制）。
package application
