// Package keys 是 rbac 的 key 唯一封装。
//
// 铁律（docs/rules/namespaces.md）：业务代码禁止裸 key 字符串，一律 import 本包。
// rbac 为 DB 装载型模块（Casbin 策略数据在 DB，无缓存/协调 key 消费者）；
// builder 随首个 key 需求添加（格式 <root>:<env>:rbac:<feature>:<id>）。
package keys
