// Package requestid provides a context-based request_id propagation helper.
//
// request_id 全链路传播（fix-fundamental-design-flaws Task 8）：
//   - middleware.RequestID 注入到 c.Request.Context()，service/repository 通过 ctx 读取
//   - 跨 goroutine 边界（PubSub / LogBus / Task）通过序列化 Meta / RequestID 字段传递，
//     worker 在执行前用 WithRequestID 恢复到子 ctx
//
// 与 gin.Context 的 c.Set("requestID", ...) 并存：
//   - 老代码（c.GetString("requestID")）保持兼容
//   - 新代码（slogutil.LoggerFromContext(ctx) / 直接 FromContext(ctx)）从 ctx 读取
package requestid

import "context"

// ctxKeyRequestID 是 request_id 在 context.Context 中的 key 类型。
// 用未导出的 struct 而非 string，避免与其他包的 key 冲突（Go 推荐范式）。
type ctxKeyRequestID struct{}

// MetaKey 是 request_id 在 PubSub Message.Meta / Task payload 中的标准字段名。
// 跨 goroutine / 跨节点序列化时统一使用此 key。
const MetaKey = "request_id"

// WithRequestID 返回一个携带 request_id 的派生 context。
// id 为空时直接返回原 ctx，避免空值污染日志字段。
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyRequestID{}, id)
}

// FromContext 从 ctx 提取 request_id；不存在时返回空字符串（不返回 error，便于日志调用）。
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, ok := ctx.Value(ctxKeyRequestID{}).(string)
	if !ok {
		return ""
	}
	return v
}
