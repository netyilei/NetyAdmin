// Package api 是 message 模块的跨模块端口（下游模块 import 本包即可消费
// 通知能力；禁止 import application/domain）。
package api

import (
	"context"
	"errors"
)

// Sender 是通知发送端口（注册名 `message.Sender`）：
//
//	sender, err := module.ResolveAs[messageapi.Sender](registry.Services, "message.Sender")
//	id, err := sender.Send(ctx, tpl.ID, "alice@example.com", map[string]string{"code": "123456"})
//
// 渠道由模板声明（email/sms；站内信走 SendInbox），目标格式随渠道
// （邮箱/手机号/用户 ID）。模板缺失/禁用时返回错误——用本包哨兵判定
// （errors.Is(err, messageapi.ErrTemplateDisabled)），不要匹配错误文本。
type Sender interface {
	// Send 按模板发送，返回发送记录 ID（可查询/重发）。
	Send(ctx context.Context, templateID, target string, vars map[string]string) (id string, err error)
	// SendInbox 直接投递站内信（不走模板），返回站内信 ID。
	SendInbox(ctx context.Context, userID, title, body string) (id string, err error)
}

// Templates 是模板自举端口（注册名 `message.Templates`）——下游模块声明式
// 创建自己的模板（无需管理端预建，也不直接写 message 私有表）：
//
//	tplReg, err := module.ResolveAs[messageapi.Templates](registry.Services, "message.Templates")
//	tpl, err := tplReg.Ensure(ctx, messageapi.TemplateSpec{
//	    Code:    "silentorder.verify_email",        // 业务唯一键（=模板 name）
//	    Channel: messageapi.ChannelEmail,
//	    Subject: "验证码",
//	    Body:    "您的验证码是 {{code}}，5 分钟内有效。",
//	})
//	id, err := sender.Send(ctx, tpl.ID, to, map[string]string{"code": code})
//
// 幂等语义：Code 不存在=创建（enabled）；已存在=返回既有行、不改写内容/状态
// （管理端编辑与禁用为权威）；Code 已存在但 Channel 不符=错误（防按错误通道
// 静默发送）。多副本并发自举收敛到同一行（name 唯一约束兜底）。
//
// 调用时机：repo 在模块 AttachDB 后可用——首次发送前按需调用（惰性），或注册
// after_bootstrap 生命周期钩子（优先级低于 db-connect:10）。
type Templates interface {
	// Ensure 按 Code 幂等确保模板存在并返回（创建或既有）。
	Ensure(ctx context.Context, spec TemplateSpec) (Template, error)
}

// TemplateSpec 是自举模板声明（Code 即持久化 name；Body 支持 {{key}} 占位符）。
type TemplateSpec struct {
	Code    string // 业务唯一标识：建议 "<module>.<purpose>"，[a-z0-9._-] 1..128
	Channel string // ChannelEmail | ChannelSms
	Subject string // email 必填；sms 留空
	Body    string // 必填；{{key}} 由 Send 的 vars 渲染
}

// Template 是模板投影（状态 1=enabled / 2=disabled）。
type Template struct {
	ID      string
	Code    string // =持久化 name
	Channel string
	Subject string
	Body    string
	Status  int16
}

// 通道常量（与持久化取值同源——下游禁字面量）。
const (
	ChannelEmail = "email"
	ChannelSms   = "sms"
)

// 模板状态常量（与持久化取值同源；Ensure 不改写状态——管理端禁用为权威）。
const (
	TemplateStatusEnabled  int16 = 1
	TemplateStatusDisabled int16 = 2
)

// 端口错误哨兵：下游用 errors.Is 判定（api 包不依赖本模块 domain——
// 模块边界 port.go 负责 domain 哨兵 → 本组哨兵的映射；两侧同文本便于排障）。
var (
	// ErrTemplateNotFound 模板 ID/Code 不存在。
	ErrTemplateNotFound = errors.New("message: template not found")
	// ErrTemplateConflict 模板 name 冲突（并发自举已由 Ensure 内部收敛，正常不外露）。
	ErrTemplateConflict = errors.New("message: template name conflict")
	// ErrTemplateDisabled 模板被管理端禁用（Ensure 不会重新启用）。
	ErrTemplateDisabled = errors.New("message: template disabled")
	// ErrTemplateCodeInvalid 自举 code 格式非法。
	ErrTemplateCodeInvalid = errors.New("message: template code invalid")
	// ErrTemplateSpecInvalid 自举参数非法（通道/主题/正文不一致）。
	ErrTemplateSpecInvalid = errors.New("message: template spec invalid")
	// ErrNoActiveDriver 通道无可用发送驱动（配置/模块问题）。
	ErrNoActiveDriver = errors.New("message: no active driver for channel")
	// ErrNotReady 服务未就绪（repo 未接——装配期误用；按可重试处理）。
	ErrNotReady = errors.New("message: service not ready (repo not attached)")
)
