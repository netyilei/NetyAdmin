// Package transport 将请求 Hook 阶段（RequestReceived/RequestAuthenticated/BeforeHandler/
// AfterHandler/ResponseSent/RequestError）接入 transport 管线。
// 边界：中间件管传输层关注点（固定顺序）；Hook 只做跨模块横切扩展
// （租户解析/审计补充/指标），轻量无阻塞；AfterHandler/ResponseSent 禁写业务 DB。
// 实现：TR-05（依赖 hooks）。
package transport
