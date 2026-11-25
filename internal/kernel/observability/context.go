// context.go：请求上下文传播——request_id/trace_id 全链路贯穿
// （HTTP → Port → RPC → Bus → Job；上下文传播规则）。
// OTel SDK 接线随（当前 W3C traceparent 做最小解析/透传）。
package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
)

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyTraceID
)

// NewRequestID 生成请求 ID（ULID，时间有序）。
func NewRequestID() string { return primitives.NewULID(primitives.SystemClock{}) }

// WithRequestID 注入请求 ID。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

// RequestIDFrom 取请求 ID（无则空串）。
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}

// WithTraceID 注入追踪 ID。
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyTraceID, id)
}

// TraceIDFrom 取追踪 ID（无则空串）。
func TraceIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(keyTraceID).(string)
	return v
}

// EnsureIDs 保证 ctx 带有 request_id/trace_id（缺失则生成）：
// trace_id 优先取上游 W3C traceparent（保留跨进程同一 trace）。
func EnsureIDs(ctx context.Context, traceparent string) (context.Context, string, string) {
	reqID := RequestIDFrom(ctx)
	if reqID == "" {
		reqID = NewRequestID()
		ctx = WithRequestID(ctx, reqID)
	}
	traceID := TraceIDFrom(ctx)
	if traceID == "" {
		if tid, ok := parseTraceparent(traceparent); ok {
			traceID = tid
		} else {
			traceID = NewRequestID()
		}
		ctx = WithTraceID(ctx, traceID)
	}
	return ctx, reqID, traceID
}

// parseTraceparent 最小 W3C 解析：version-traceid-spanid-flags，取 traceid。
// 格式：00-<32hex>-<16hex>-01
func parseTraceparent(header string) (string, bool) {
	const totalLen = 55 // 2+1+32+1+16+1+2
	if len(header) != totalLen || header[2] != '-' || header[35] != '-' || header[52] != '-' {
		return "", false
	}
	traceID := header[3:35]
	for _, c := range traceID {
		if !isHex(byte(c)) {
			return "", false
		}
	}
	if traceID == "00000000000000000000000000000000" {
		return "", false // 全零无效（W3C 规范）
	}
	return traceID, true
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// NewSpanID 生成 16 hex span ID（OTel 接线前的占位生成）。
func NewSpanID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}
