// Package transport 提供 HTTP 传输管线（Gin）：
// 固定顺序中间件链（RequestID → Metrics → Recovery → BodyLimit →
// SecurityHeaders → CORS → AccessLog）、统一信封渲染（api-style §4）、
// 健康端点与 /metrics。edges 不 import 业务模块——业务路由
// 经装配桥接 RouteCollector。
package transport
