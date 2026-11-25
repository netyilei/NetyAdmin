// Package keys 是 audit 的 key 唯一封装。
//
// 铁律（docs/rules/namespaces.md）：业务代码禁止裸 key 字符串，一律 import 本包。
// audit 为 DB 落库型模块（无缓存/协调 key 消费者）；builder 随首个 key 需求添加
// （格式 <root>:<env>:audit:<feature>:<id>；快照测试在此登记）。
package keys
