// Package events 是 rbac 的事件 topic 唯一封装（§5.5）。
// 策略变更广播：管理面写路径变更角色/授权后发布 → 各节点 Casbin
// reload（tech-stack §4 多机同步；不依赖 casbin Redis watcher）。
package events

// TopicPolicyChanged 是 RBAC 策略变更广播（payload 无关——订阅方收到即全量
// reload；幂等可重复触发）。
const TopicPolicyChanged = "platform.rbac.changed"
