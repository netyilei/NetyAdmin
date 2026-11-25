// Package events 是 configcenter 的事件 topic 唯一封装（§5.5）。
// 配置变更广播：管理面写路径变更事务提交后发布 → 各节点 Watcher
// reload（AGENTS §4 多机热更；不依赖 DB 轮询）。
package events

// TopicConfigChanged 是动态配置变更广播（payload = {"version": N}——订阅方
// 收到即全量 reload；幂等可重复触发）。
const TopicConfigChanged = "platform.config.changed"
