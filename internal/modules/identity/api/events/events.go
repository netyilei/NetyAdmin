// Package events 是 identity 的事件 topic 唯一封装（§5.5）。
// 登录审计事件：SecurityRecord 归属 audit，identity 只发布——
// audit 模块订阅落 operation_logs（接线；payload 见 TopicLoginAudit 注）。
package events

// TopicLoginAudit 是登录审计事件（domain.identity.login.audit）。
// payload = identity LoginAuditEvent JSON（success/failed 两种 action 共用）；
// audit 模块订阅后转 SecurityRecord 落 operation_logs。
const TopicLoginAudit = "domain.identity.login.audit"

// TopicTokenRevoked 是 token 全作废事件（domain.identity.token.revoked）。
// TokenVersion bump（改密/重置/禁用）成功后发布——多机下他节点即时感知会话
// 作废（事件化；当前无订阅方=仅目录留痕，消费方接入后无需改
// 发布面）。payload = {"admin_id":"<id>","reason":"password_change|password_reset|account_disabled"}。
const TopicTokenRevoked = "domain.identity.token.revoked"
