// Package adapters 是 note 适配器层：domain 仓储接口实现、外部系统适配（SMTP/OSS/CDN/SMS/MQ）。
//
// 允许：实现 domain 仓储接口与 api/ 端口；import 基础设施客户端（GORM/pgx/go-redis）。
// 禁止：发明状态码（外部错误只做转换，映射在 application）；跨模块 import internal。
package adapters
