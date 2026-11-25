// Package coordination 实现 Coordination 平面：锁、限流、Nonce、幂等、选主、计数器。
// 策略唯一源 contracts/state-policies.yaml（coordination/rate_limit 段），Go 常量为
// 其镜像；每原语 fail-open / fail-closed / 单机 local 显式声明，本地回退仅
// topology=single 允许（multi 统一把守）。
//
// 行为模式：Redis 操作失败 → decideUnavailable 按策略分发（无前置探活，操作错误
// 即不可用信号）；键前缀 coord:* 具名常量。
// 登记缺口：leader multi 的 PG advisory 分支随；idempotency single 的
// DB 分支随幂等表；降级指标（coordination_fallback_total）随 OBS；
// contracts↔镜像 CI 对偶校验待补。
package coordination
