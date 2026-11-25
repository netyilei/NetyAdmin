// envelope.go：WS 消息信封（ws-protocol §3 逐字镜像——字段/必填/约束）。
// 未知字段必须忽略（向前兼容）——客户端侧约束；服务端产出只含下列字段。
package protocol

import (
	"encoding/json"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
)

// SchemaVersion 是负载 schema 版本（§3 必填；只加字段不破坏）。
const SchemaVersion = 1

// Envelope 是 WS 帧信封（§3；error 帧的 code=5 位业务码——状态码单一源
// errorx，禁三位 HTTP 码；close 帧的 close_code=§5 关闭码）。
type Envelope struct {
	ID            string          `json:"id"`                // ULID；消息唯一 ID（幂等/去重）
	Type          string          `json:"type"`              // §4 帧类型常量
	Channel       string          `json:"channel,omitempty"` // 订阅/消息类必填
	Payload       json.RawMessage `json:"payload,omitempty"` // 业务负载 ≤64KB
	TS            int64           `json:"ts"`                // 服务端毫秒时间戳
	RequestID     string          `json:"request_id,omitempty"`
	TraceID       string          `json:"trace_id,omitempty"`
	SchemaVersion int             `json:"schema_version"`        // 只加字段
	Seq           int64           `json:"seq,omitempty"`         // 频道内序号（排序/续传）
	Code          string          `json:"code,omitempty"`        // error 帧：5 位业务码
	MessageKey    string          `json:"message_key,omitempty"` // error 帧：i18n 键
	CloseCode     int             `json:"close_code,omitempty"`  // close 帧：§5 关闭码
	Reason        string          `json:"reason,omitempty"`      // close 帧：原因
}

// New 构造服务端产出帧（ts=now 毫秒；schema_version 固定）。
func New(typ string) Envelope {
	return Envelope{Type: typ, TS: time.Now().UnixMilli(), SchemaVersion: SchemaVersion}
}

// Marshal 编码为发送帧（session 写入面统一出口——单帧 ≤64KB 由调用方校验）。
func (e Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// —— error 帧业务码（ws-protocol §4 各面段——同语义同位）——
// 登记面 contracts/status-codes.yaml：admin 22xxx / gateway 185xx / client 32xxx
// （surface: *.ws, domain: ws；生成常量经 errorx 单一源引用，禁止字面量）。

// wsErrorCodes 是 surface→语义键→登记码映射（session sendError 消费）。
var wsErrorCodes = map[string]map[string]string{
	"admin": {
		"realtime.channel_denied":     errorx.CodeAdminWSChannelDenied,
		"realtime.invalid_channel":    errorx.CodeAdminWSInvalidChannel,
		"realtime.protocol_violation": errorx.CodeAdminWSProtocolViolation,
		"realtime.channel_limit":      errorx.CodeAdminWSChannelLimit,
		"realtime.resume_unavailable": errorx.CodeAdminWSResumeUnavailable,
		"realtime.not_subscribed":     errorx.CodeAdminWSNotSubscribed,
	},
	"gateway": {
		"realtime.channel_denied":     errorx.CodeGatewayChannelDenied,
		"realtime.invalid_channel":    errorx.CodeGatewayWSInvalidChannel,
		"realtime.protocol_violation": errorx.CodeGatewayWSProtocolViolation,
		"realtime.channel_limit":      errorx.CodeGatewayWSChannelLimit,
		"realtime.resume_unavailable": errorx.CodeGatewayWSResumeUnavailable,
		"realtime.not_subscribed":     errorx.CodeGatewayWSNotSubscribed,
	},
	"client": {
		"realtime.channel_denied":     errorx.CodeClientWSChannelDenied,
		"realtime.invalid_channel":    errorx.CodeClientWSInvalidChannel,
		"realtime.protocol_violation": errorx.CodeClientWSProtocolViolation,
		"realtime.channel_limit":      errorx.CodeClientWSChannelLimit,
		"realtime.resume_unavailable": errorx.CodeClientWSResumeUnavailable,
		"realtime.not_subscribed":     errorx.CodeClientWSNotSubscribed,
	},
}

// ErrorFrameCode 是 WS error 帧的业务码（surface+语义键 → 登记码；未登记组合
// 回退核心内部错误 10009）。
func ErrorFrameCode(surface, semanticKey string) string {
	if m, ok := wsErrorCodes[surface]; ok {
		if code, ok := m[semanticKey]; ok {
			return code
		}
	}
	return errorx.CodeInternal
}
