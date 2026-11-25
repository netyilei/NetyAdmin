// Package kernel 承载 L0 运行时内核与 L1 全局子系统，是全平台唯一的公共底座。
//
// 依赖规则（§3）：
//   - kernel 不 import internal/modules，不出现任何业务名词；
//   - 平台/业务/接入面只允许使用 kernel 声明的公共 API；
//   - 每个子系统先 local/memory 实现，Redis/远程作为可选适配器。
//
// 子系统与实现里程碑：
//
//	-（..）：config/bootstrap/module/hooks/database/state/log/errorx/
//	  observability/transport/security(primitives)；
//	-（BUS/SCH）：bus/scheduler；
//	-（SEC/TR/OBS）：security 完整面、transport/rpc、transport/ws。
package kernel
