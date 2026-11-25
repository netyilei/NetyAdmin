# 可靠性基线（reliability）

> 适用：kernel 全部子系统与所有模块的外部交互。状态降级的唯一源是 contracts/state-policies.yaml（见 state-and-cache.md），本文件只定降级之外的超时/重试/熔断/幂等/恢复约定。

## 1. 超时（一切外部调用必须显式超时，禁止依赖默认值）

> 下表为**初始基线**，模块/部署配置可覆盖，但覆盖值必须显式声明；「事务 5s」出自 migrations.md 事务行。

| 调用 | 基线超时 | 备注 |
|------|---------|------|
| 内部 RPC | 按端口方法元数据（Manifest 声明） | 客户端不得自行放大 |
| 外部 HTTP API | 5s | SMTP/OSS/短信等适配器各自可覆盖，须写配置 |
| DB 单查询 | 5s | 事务总超时 5s（migrations.md「事务」行） |
| Redis 操作 | 500ms | 协调类原语按策略表 |
| 优雅关闭在途等待 | 5s | bootstrap 生命周期 |

- 超时必须随 context 传播；禁止 `context.Background()` 发起外部调用。

## 2. 重试

- **只重试幂等操作**；非幂等写不重试（重试由幂等键保护后放开）。
- 指数退避 + jitter（cenkalti/backoff/v5）；单调用链重试 ≤3 次；重试计入调用方超时预算。
- 重试不得放大故障：下游已熔断/限流 429 时直接失败，不叠加重试。

## 3. 熔断

- 内部 RPC 与外部 API 一律挂 sony/gobreaker/v2；按目标（host/port）维度隔离。
- 半开探测成功数 ≥3 才全开恢复（初始基线，gobreaker 配置项可调）；熔断态必须出指标并告警（observability.md）。

## 4. 幂等（不承诺 exactly-once 的世界里的自保）

| 场景 | 幂等机制 |
|------|---------|
| 管理端/C 端写操作 | `Idempotency-Key` 头（api-style §5）+ 业务唯一约束 |
| 调度 Job | 业务唯一键 + 运行记录先写 running |
| 事件消费 | Inbox 表/Redis 去重 + 业务幂等键 |
| Outbox 投递 | 消息 ID 全局唯一，消费方按 ID 去重 |

- 所有至少一次通道（Bus Cluster、River、重试）的消费者**默认按会重复投递设计**。

## 5. 恢复

- leader 失联 → 租约过期后新 leader 接管；崩溃时的在途 Job 标记 unknown 按策略补跑。
- 服务重启后 Coordination/Session 平面按策略表恢复（state-and-cache.md），禁止"内存即真相"。
- 每个降级动作必须有恢复路径与告警（降级静默 = 违规）。

## 6. 反模式（CI/评审红线）

- 吞错（`_ = err`）、无限重试、无超时外部调用、降级不告警、以 panic 做流程控制。

> 网关 quota Rate=0 语义：显式无限（`Rate<=0` 三处代码一致）——契约定稿见 docs/guide/gateway-canonical.md。
