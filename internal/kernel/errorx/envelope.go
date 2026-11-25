package errorx

import "errors"

// Envelope 是三个 HTTP 接入面统一的响应信封（api-style.md §4；字段名与契约登记一致）。
// 权威定义：docs/rules/status-codes.md §8；契约面：contracts/openapi/*.yaml 的 Envelope schema。
type Envelope struct {
	Code       string         `json:"code"`                 // 5 位字符串状态码
	MessageKey string         `json:"message_key"`          // i18n 文案键（禁硬编码文案）
	Data       any            `json:"data,omitempty"`       // 业务数据
	Details    map[string]any `json:"details,omitempty"`    // 错误结构化上下文
	RequestID  string         `json:"request_id,omitempty"` // 请求链路
	TraceID    string         `json:"trace_id,omitempty"`   // 追踪链路
}

// NewEnvelope 构造成功信封（Code=10000）。
func NewEnvelope(data any) *Envelope {
	return &Envelope{Code: CodeSuccess, MessageKey: MessageKeyOf(CodeSuccess), Data: data}
}

// NewErrorEnvelope 从 *Error 构造失败信封。
func NewErrorEnvelope(e *Error, requestID, traceID string) *Envelope {
	return &Envelope{
		Code:       e.Code(),
		MessageKey: e.MessageKey(),
		Details:    e.Details(),
		RequestID:  requestID,
		TraceID:    traceID,
	}
}

// NewErrorEnvelopeOf 从任意 error 构造失败信封。
// 用 errors.As 穿透 %w 包装链提取 *Error；非 *Error 归一为内部错误（10009，不外泄底层细节）。
func NewErrorEnvelopeOf(err error, requestID, traceID string) *Envelope {
	var e *Error
	if errors.As(err, &e) {
		return NewErrorEnvelope(e, requestID, traceID)
	}
	return NewErrorEnvelope(New(CodeInternal), requestID, traceID)
}
