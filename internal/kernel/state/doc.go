// Package state 是状态子系统：cache（性能）/ coordination（正确性）/ session（安全）
// 三平面 + 模式解析与健康诊断。
// L1 常开无开关；L2 多机必开；entity 只声明 class（hot_local/shared/durable），禁止手工选层。
// 策略唯一源：contracts/state-policies.yaml；规范：docs/rules/state-and-cache.md。
// 实现：///。
package state
