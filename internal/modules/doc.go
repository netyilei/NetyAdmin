// Package modules 承载 L2 平台服务、L3 业务模块与 L4 接入面。
//
// 模块标准结构：
//
//	api/（唯一对外 import 面）· domain/ · application/ · adapters/ ·
//	module.go（仅装配）· module.yaml（Manifest）· migrations/
//
// 已登记平台模块（contracts/namespaces.yaml）：identity rbac configcenter audit ipac
// storage message content realtime gateway scheduler。
// 接入面（edge）：adminapi clientapi gateway ops ——/ 落地。
// 模块骨架由脚手架脚本统一生成，禁止手工散建。
package modules
