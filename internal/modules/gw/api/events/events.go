// Package events 是 gw 的事件 topic 唯一封装。
// 治理变更事件（app/scope/API 白名单写入→edge 判定快照失效）与 IP 规则变更
// 同惯例：本节点 Reload + DomainBroadcast 广播，各节点订阅后全量 Reload。
package events

// TopicRulesChanged 是 IP 策略组/规则变更事件（——变更广播全节点
// reload；DomainBroadcast 域。命名守 Bus 域 topic 4 段约束：
// domain.<module>.<topic>.<verb>）。
const TopicRulesChanged = "domain.gw.rules.changed"

// TopicAppsChanged 是 edge 判定快照变更事件（「edge HMAC 热路径
// 应用快照装载化」——app/scope 授权/API 登记/scope 白名单任一写入 commit 后
// 广播；订阅方 reload 应用快照收敛）。
const TopicAppsChanged = "domain.gw.apps.changed"
