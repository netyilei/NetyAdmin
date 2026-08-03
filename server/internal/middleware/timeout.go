package middleware

import (
	"net/http"
	"strings"
	"time"
)

// TimedOutBody 是 http.TimeoutHandler 触发超时时返回的响应体。
// 使用专属业务错误码 100011（errorx.CodeRequestTimeout），与限流码 100006
// （errorx.CodeTooManyRequest，"请求过于频繁"）区分，使客户端可仅凭 code 字段
// 区分「请求超时」与「请求过于频繁」两种语义。
const TimedOutBody = `{"code":"100011","msg":"请求超时"}`

// eventStreamMIME 是 SSE（Server-Sent Events）的媒体类型。
// 见 WrapWithTimeout：携带该 Accept 的请求按流式响应豁免请求超时。
const eventStreamMIME = "text/event-stream"

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
//   - **SSE/流式响应豁免**：http.TimeoutHandler 的 timeoutWriter 是缓冲语义——
//     将 handler 写入的响应先缓冲、待 handler 返回后才刷到真正的 ResponseWriter。
//     SSE 长连接（handler 永不返回）会导致客户端收不到任何字节，直至超时被 503 掐断。
//     因此对 Accept: text/event-stream 的请求不走 TimeoutHandler，直接透传原 handler，
//     长连接由 Server.WriteTimeout（连接层）兜底，不拦截流式输出。
func WrapWithTimeout(handler http.Handler, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		// timeout 为零值时不启用超时包装，直接返回原 handler（避免 http.TimeoutHandler 把 0 当「立即超时」）。
		return handler
	}
	tw := http.TimeoutHandler(handler, timeout, TimedOutBody)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SSE/流式响应豁免：长连接不适用请求级超时（TimeoutHandler 缓冲会吞掉流式输出，见上方注释）。
		// 用 Accept 媒体类型协商匹配（非裸子串），避免恶意客户端用 "text/event-stream" 子串
		// 混入 Accept 绕过超时（DoS 面）。匹配 media-range 边界（逗号/分号/空格分隔）。
		if acceptsEventStream(r.Header.Get("Accept")) {
			handler.ServeHTTP(w, r)
			return
		}
		tw.ServeHTTP(w, r)
	})
}

// acceptsEventStream 检查 Accept 头是否包含精确的 "text/event-stream" 媒体类型。
// 不用 strings.Contains——子串匹配会把 "application/json, text/event-stream-fake" 误判。
// 这里按 RFC 7231 media-range 边界（逗号分隔条目，分号分隔参数）精确比对。
func acceptsEventStream(accept string) bool {
	for _, item := range strings.Split(accept, ",") {
		// 取类型部分（分号前），去 OWS（可选空白）
		mediaType := strings.TrimSpace(strings.SplitN(item, ";", 2)[0])
		if strings.EqualFold(mediaType, eventStreamMIME) {
			return true
		}
	}
	return false
}
