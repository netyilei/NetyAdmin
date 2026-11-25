// Package keys 是 gw 的 key 唯一封装。
//
// 铁律（docs/rules/namespaces.md）：业务代码禁止裸 key 字符串，一律 import 本包。
// gw 为 DB 装载型模块（无缓存/协调 key 消费者）；nonce 防重放 key 归
// edge 面（格式 <root>:<env>:gw:nonce:<appkey>:<nonce>）。
package keys

// GroupNonce 是 edge nonce 防重放的 Redis 键组前缀。
const GroupNonce = "nonce"

// 统计面（——查询端点统一权限码；流水/快照全文件化 2026-11-19）。
const PermStatsView = "admin.gw.stats.view"
const (
	PermIPGroupsList  = "admin.gw.ip-groups.list"
	PermIPGroupsWrite = "admin.gw.ip-groups.write"
	PermIPRulesList   = "admin.gw.ip-rules.list"
	PermIPRulesWrite  = "admin.gw.ip-rules.write"
	PermIPReload      = "admin.gw.ip-reload"
)

// 接口权限页（scope 组字典管理——组 CRUD；API 白名单复用 admin.gw.apis.*）。
const (
	PermScopeGroupsList  = "admin.gw.scope-groups.list"
	PermScopeGroupsWrite = "admin.gw.scope-groups.write"
)
