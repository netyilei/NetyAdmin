// Package ratelimit 实现统一限流策略引擎：维度（ip/user/app/route/global）×
// 算法 × 失败模式（open/closed）。策略名/失败模式唯一源 contracts/state-policies.yaml
// （镜像 + CI 对偶校验）；算法当前唯一实现=滑动窗口（Lua；tech-stack §9
// 已否固定窗口库），token_bucket/fixed_window 随具名消费者扩展（登记）。
// 一个中间件 + 策略注册表；配额/并发/WS 连接数在同一模型下扩展（随消费方）。
// 实现：（依赖 state/coordination）。
package ratelimit
