package message

import "context"

// Driver 消息发送驱动接口
type Driver interface {
	// Send 发送消息
	// ctx: 上下文，包含超时控制
	// receiver: 接收人标识 (手机号/邮箱/Token)
	// title: 消息标题 (仅部分通道支持)
	// content: 消息正文
	// params: 扩展参数
	Send(ctx context.Context, receiver string, title string, content string, params map[string]string) error
}

// SmsDriver 短信专用驱动接口
type SmsDriver interface {
	Driver
	// SendWithTemplate 使用模板发送短信
	SendWithTemplate(ctx context.Context, phone string, templateID string, params map[string]string) error
}
