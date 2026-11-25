// message.go：消息域模型（V1 message 对齐——模板/发送记录/通道）。
package domain

import (
	"errors"
	"time"
)

// 领域错误（调用方映射状态码）。
var (
	ErrTemplateNotFound = errors.New("message: template not found")
	ErrTemplateConflict = errors.New("message: template name conflict")
	ErrRecordNotFound   = errors.New("message: record not found")
	ErrChannelInvalid   = errors.New("message: channel invalid")
	ErrNoActiveDriver   = errors.New("message: no active driver for channel")
	ErrTemplateDisabled = errors.New("message: template disabled")
	// ErrTemplateCodeInvalid 是自举模板 code 非法（业务唯一标识格式校验）。
	ErrTemplateCodeInvalid = errors.New("message: template code invalid")
	// ErrTemplateSpecInvalid 是自举模板参数非法（通道/主题/正文不一致——
	// 含 code 已存在但通道与声明不符：防下游按错误通道静默发送）。
	ErrTemplateSpecInvalid = errors.New("message: template spec invalid")
	// ErrTemplateNameImmutable：模板 name 是业务唯一键（自举 code）——管理面
	// 改名会让下游 Ensure 按原 code 静默新建第二行，故禁止改名（改文案/状态可）。
	ErrTemplateNameImmutable = errors.New("message: template name is immutable (business key)")
	// ErrNotReady：依赖未就绪（repo 未 AttachDB——装配期误用；调用方应视作可重试）。
	ErrNotReady = errors.New("message: service not ready (repo not attached)")
)

// Channel 是消息通道（ws-protocol 无关——notify 面）。
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelSms   Channel = "sms"
)

// Valid 通道合法性。
func (c Channel) Valid() bool { return c == ChannelEmail || c == ChannelSms }

// 模板状态。
const (
	TemplateEnabled  int16 = 1
	TemplateDisabled int16 = 2
)

// 发送记录状态（重试 Job 驱动 pending/failed → sent）。
const (
	RecordPending int16 = 1
	RecordSent    int16 = 2
	RecordFailed  int16 = 3
)

// MaxRetries 是异步重试上限（超限不再重投——无死信面登记）。
const MaxRetries = 3

// Template 是消息模板（占位符 {{key}} 由发送时渲染）。json tag snake_case
// （响应面契约—— 内与 storage 一致；api-style camelCase vs snake 分裂
// 裁决项随统一）。
type Template struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Channel   Channel   `json:"channel"`
	Subject   string    `json:"subject"` // sms 通道为空
	Body      string    `json:"body"`    // {{key}} 占位符
	Status    int16     `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SendRecord 是一次发送记录（流水台账；resend=基于原记录重投）。
type SendRecord struct {
	ID         string     `json:"id"`
	TemplateID string     `json:"template_id"`
	Channel    Channel    `json:"channel"`
	Target     string     `json:"target"` // 邮箱/手机号（接收方）
	Subject    string     `json:"subject"`
	Body       string     `json:"body"` // 渲染后正文
	Driver     string     `json:"driver"`
	Status     int16      `json:"status"`
	Retries    int        `json:"retries"`
	Error      string     `json:"error"`
	CreatedAt  time.Time  `json:"created_at"`
	SentAt     *time.Time `json:"sent_at"`
}
