// Package keys 是 identity 的 key 唯一封装。
//
// 铁律（docs/rules/namespaces.md）：业务代码禁止裸 key 字符串，一律 import 本包。
// identity 为 DB 落库+session 平面型模块（前缀 sess:* 由 session 包定义——
// 共享基础设施 key 已在 kernel 登记）；builder 随首个模块自有 key 需求添加
// （格式 <root>:<env>:identity:<feature>:<id>）。
package keys
