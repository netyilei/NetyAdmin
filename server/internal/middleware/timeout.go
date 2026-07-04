package middleware

import (
	"net/http"
	"time"
)

// TimedOutBody 是 http.TimeoutHandler 触发超时时返回的响应体。
// 使用专属业务错误码 100011（errorx.CodeRequestTimeout），与限流码 100006
// （errorx.CodeTooManyRequest，"请求过于频繁"）区分，使客户端可仅凭 code 字段
// 区分「请求超时」与「请求过于频繁」两种语义。
const TimedOutBody = `{"code":"100011","msg":"请求超时"}`

// WrapWithTimeout 用 http.TimeoutHandler 包装底层 http.Handler，
// 当请求处理超过 timeout 时返回 HTTP 503 + JSON 错误体。
//
// 与原 context.WithTimeout 中间件的区别：
//   - 原中间件仅向 ctx 注入 deadline，不拦截响应；
//     若 handler 忽略 ctx 取消继续执行，客户端会等到 ReadTimeout/WriteTimeout 触发后被连接层断开（空响应/重置）。
//   - http.TimeoutHandler 在新 goroutine 中执行底层 handler，
//     超时时由 stdlib 主动写入 503 + body 并立即返回，确保客户端收到结构化错误体。
//
// 注意：
//   - timeout 应略小于 Server.ReadTimeout / WriteTimeout，
//     否则连接层会先于中间件超时断开，503 错误体无法送达客户端。
//   - gin.Engine 实现了 http.Handler 接口（ServeHTTP），可直接传入。
//   - http.TimeoutHandler 内部会设置 Content-Type: text/plain; charset=utf-8，
//     body 为 JSON 字符串但 content-type 不是 application/json，
//     这是 stdlib 的限制（无法自定义 header），客户端按 body 内容解析即可。
func WrapWithTimeout(handler http.Handler, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		// timeout 为零值时不启用超时包装，直接返回原 handler（避免 http.TimeoutHandler 把 0 当「立即超时」）。
		return handler
	}
	return http.TimeoutHandler(handler, timeout, TimedOutBody)
}
