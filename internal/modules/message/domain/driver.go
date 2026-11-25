// driver.go：通知发送驱动契约（2026-11-19 设计——参考对象存储
// 「通用协议优先」：email=SMTP 标准协议驱动（内置）；sms 无通用开放协议（厂商
// 私有 API）→ 默认 log 驱动（开发/测试/演示），厂商驱动由下游按本契约自写
// （Register 面注册——基座不绑定任何厂商 SDK，G1-09）。
package domain

import "context"

// Outbound 是待发送消息（模板渲染后）。
type Outbound struct {
	To      string // 邮箱/手机号
	Subject string // sms 通道为空
	Body    string
}

// Driver 是发送驱动契约（每通道一实现集——下游 Register 注册厂商驱动）。
type Driver interface {
	// Name 是驱动标识（config 选择面；如 "smtp"/"log"）。
	Name() string
	// Send 发送一条消息（同步——异步编排/重试由 Service 承担）。
	Send(ctx context.Context, msg Outbound) error
}
