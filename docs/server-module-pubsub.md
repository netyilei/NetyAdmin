# 统一消息订阅总线 (PubSubBus) 详解

本文档详细介绍 NetyAdmin PubSubBus 模块的架构设计、驱动机制、配置方式及二次开发指南。

---

## 一、模块概述

PubSubBus 是全系统的"消息分发中心"，将原本散落在各模块的独立 Redis Pub/Sub 订阅统一收口为 **1 个连接、1 个常驻协程**，并根据消息中的 Topic 字段分发给注册的订阅者。

### 1.1 核心特性

- **驱动化设计**：支持 MemoryDriver（单机）和 RedisDriver（集群）两种实现，通过配置一键切换
- **统一频道**：所有 Topic 共用一个 Redis 频道 `{prefix}:channel:system_bus`，消息体内含 Topic 字段用于路由
- **Topic 注册表**：所有 Topic 必须在 `internal/pkg/pubsub/topics.go` 中注册，严禁硬编码
- **生命周期管理**：由 PubSubBus 统一管理订阅协程的启动与关闭，应用退出时自动清理
- **零侵入降级**：未配置 Redis 时自动降级为 MemoryDriver，单机开发无需任何依赖

### 1.2 改造收益

| 指标 | 改造前 | 改造后 |
|------|--------|--------|
| Redis 连接数 | 4（每个 Subscribe 独占 1 个） | 1（共享） |
| 常驻协程数 | 4 | 1 |
| 生命周期管理 | 分散在各模块，无统一 Stop | 由 PubSubBus 统一管理 |
| 模块耦合 | IPAC 等模块需依赖 CacheManager 获取 Redis 连接 | 仅依赖 EventBus 接口 |

---

## 二、目录结构

```
server/internal/pkg/pubsub/
├── bus.go              # EventBus 接口、Message 协议、MemoryDriver、RedisDriver
└── topics.go           # Topic 注册表（全系统唯一权威来源）
```

---

## 三、架构设计

### 3.1 逻辑架构图

```text
[Redis Pub/Sub]  或  [Memory Channel]
      |                    |
      v                    v
[PubSubBus 消费 loop (1 个常驻协程)]
      |
      | dispatch(ctx, msg)
      v
[dispatchQueue (buffered channel, default 1024)]
      |
      +--- worker 1 ---+--- topic: config_sync --------> [ConfigWatcher.ForceReload()]
      +--- worker 2 ---+--- topic: storage_sync -------> [StorageConfigService.LoadAllConfigs()]
      +--- ...       ---+--- topic: cache_invalidation -> [CacheLifecycle.InvalidateL1ByTags()]
      +--- worker N ---+--- topic: ipac_reload --------> [IPACService.ReloadCache()]
```

### 3.2 EventBus 接口

```go
type EventBus interface {
    Publish(ctx context.Context, topic string, msg interface{}) error
    Subscribe(topic string, handler func(ctx context.Context, msg []byte)) error
    Close() error
    // OnReconnect 注册 Redis 重连成功后的回调（Task 13.2 / 13.3 兜底机制）。
    // 仅 RedisDriver 会在 subscribeLoop 从断连恢复后触发；MemoryDriver 无重连概念，
    // 注册的回调永远不会被调用。可选：若不注册，subscribeLoop 重连后正常运行（no-op）。
    // 回调在独立 goroutine 中执行（GoSafe 包裹），不会阻塞订阅协程。
    OnReconnect(fn func())
}
```

| 方法 | 说明 |
|------|------|
| `Publish` | 向指定 Topic 发布消息，msg 会被序列化为 JSON。**失败时调用方应 slog.Error 上报，不得静默吞错**（Task 13.1） |
| `Subscribe` | 订阅指定 Topic，handler 接收的是消息的 Payload 原始字节 |
| `Close` | 关闭总线，停止订阅协程并释放资源；先排空 dispatchQueue 再关闭 worker pool（Task 23） |
| `OnReconnect` | 注册 Redis 重连成功后的回调（可选，未注册则 no-op）；仅 RedisDriver 会触发 |

### 3.3 消息协议

所有消息通过统一频道传输，消息体结构：

```json
{
    "topic": "config_sync",
    "payload": "<JSON 编码的业务数据>",
    "timestamp": 1713715200,
    "senderId": "host-ULID8",
    "meta": {"request_id": "..."}
}
```

- `topic`：消息主题，用于路由到对应的订阅者
- `payload`：业务数据（`json.RawMessage`），由发布者序列化、订阅者反序列化
- `timestamp`：Unix 时间戳
- `senderId`：发布节点 ID（Task 5），接收方据此过滤本节点回环
- `meta`：跨 goroutine / 跨节点传递的上下文元数据（Task 8），目前含 `request_id`，nil 表示无元数据（向后兼容）

### 3.4 Topic 注册表

**强制规范**：严禁在业务代码中硬编码 Topic 字符串，必须在 `internal/pkg/pubsub/topics.go` 中统一定义。

当前注册的 Topic：

| Topic 常量 | 值 | 发布者 | 订阅者 | 说明 |
|------------|-----|--------|--------|------|
| `TopicConfigSync` | `"config_sync"` | ConfigService | ConfigWatcher | 系统配置变更广播 |
| `TopicStorageSync` | `"storage_sync"` | StorageConfigService | StorageConfigService | 存储配置变更广播 |
| `TopicCacheInvalidation` | `"cache_invalidation"` | CacheLifecycle | CacheLifecycle | 分布式缓存失效同步 |
| `TopicIPACReload` | `"ipac_reload"` | IPACService | IPACService | IP 规则重载通知 |

---

## 四、驱动实现

### 4.1 MemoryDriver（单机模式）

基于 Go 原生 `channel` 和 `sync.Map` 实现进程内事件分发。

```text
Publish ──► msgChan (buffered 1000) ──► loop() ──► dispatch(msg)
                                                    │
                                                    v
                                       dispatchQueue (buffered)
                                                    │
                                       +------------+------------+
                                       |            |            |
                                    worker1      worker2      workerN
                                       |            |            |
                                       v            v            v
                                  handler(msg)  handler(msg)  handler(msg)
```

**特点**：
- 无需部署 Redis，极轻量
- 消息仅在当前进程内传播，不支持跨节点
- 适合开发环境或单机部署
- 与 RedisDriver 共享 dispatch worker pool 设计（Task 23）

### 4.2 RedisDriver（集群模式）

基于 Redis Pub/Sub 实现，订阅统一频道 `{prefix}:channel:system_bus`。

```text
Publish ──► redisClient.Publish(channel, message)
                         │
                    Redis Server
                         │
                         v
              subscribeLoop() ──► dispatch(msg)
                                    │
                                    v
                          dispatchQueue (buffered)
                                    │
                          +---------+---------+---------+
                          |         |         |
                       worker1   worker2   workerN
                          |         |         |
                          v         v         v
                     handler(msg) handler(msg) handler(msg)
```

**特点**：
- 支持多节点间的广播通信
- 保证分布式缓存和配置的一致性
- 仅占用 1 个 Redis 连接
- 与 MemoryDriver 共享 dispatch worker pool 设计（Task 23）

### 4.3 驱动选择策略

通过 `config.toml` 的 `[bus] driver` 配置控制：

| 配置值 | 行为 |
|--------|------|
| `"redis"` | 强制使用 RedisDriver，若 Redis 未启用则启动报错 |
| `"memory"` | 强制使用 MemoryDriver，即使 Redis 可用也不使用 |
| 不设置（默认） | 根据 `redis.enabled` 自动选择：Redis 启用 → RedisDriver，否则 → MemoryDriver |

### 4.4 dispatch worker pool（Task 23）

dispatch 阶段采用 **buffered channel + N workers** 模式，替代原本的 per-event goroutine（每条消息一个 goroutine）。worker pool 在 driver 构造时创建（`initWorkerPool`），不会延迟到首次 Publish。

#### 设计目标

- **避免 goroutine 数量爆炸**：高吞吐场景下原方案会为每条消息创建一个 goroutine，瞬时 goroutine 数量等于消息速率 × handler 数。worker pool 将并发度限制为固定 N。
- **背压（backpressure）**：队列满时 dispatch 阻塞，让消费 loop 反压到上游 Publish（MemoryDriver）/ Redis Pub/Sub 投递（RedisDriver，但 Redis 端无 backpressure）。
- **panic 隔离**：per-handler `defer/recover` 防止单个 handler panic 跳过同消息后续 handler 或杀死 worker。

#### 关键参数

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `[pubsub].workers` | 16 | worker goroutine 数量，零值兜底为 16 |
| `[pubsub].queue_size` | 1024 | `dispatchQueue` 缓冲容量，零值兜底为 1024 |

环境变量覆盖：`NETYADMIN_PUBSUB_WORKERS`、`NETYADMIN_PUBSUB_QUEUE_SIZE`。

#### 数据流

```text
消费 loop (loop / subscribeLoop)
    │
    │ dispatch(ctx, msg)              // ctx 由 msg.Meta 恢复 request_id
    │
    │ select {
    │   case dispatchQueue <- job:
    │   case <-dispatchStop:          // 关闭中：丢弃消息
    │ }
    │
    v
dispatchQueue  ──►  worker (range)
                       │
                       │ invokeHandlers(ctx, msg)
                       │
                       │ for h := range handlers {
                       │   func(h) {
                       │     defer recover()           // per-handler 隔离
                       │     h(ctx, msg.Payload)
                       │   }(h)
                       │ }
                       v
                    [handler1, handler2, ..., handlerN]
```

#### 队列满时的行为（backpressure）

默认行为：**block**（更安全的语义）。

- `dispatchQueue <- job` 在队列满时阻塞，直到有 worker 消费一条消息。
- 阻塞会反压到消费 loop（`MemoryDriver.loop` / `RedisDriver.subscribeLoop`）。
- MemoryDriver 进一步反压到 `Publish`（`msgChan` 满后 `Publish` 阻塞或返回 `ctx.Err()`）。
- RedisDriver 无法反压到 publisher（Redis Pub/Sub 是 fire-and-forget），但能避免消费 loop 跑飞导致 dispatchQueue 无限增长。

如未来需要"日志告警 + drop"语义（避免 Publish 路径反压），可在 `dispatch` 增加 `default` 分支：`slog.Warn("dispatch queue full, dropping")`。本 spec 不实现该模式，默认 block 更安全。

#### 优雅关闭（graceful shutdown）

`Close()` 的关闭顺序（两个 driver 一致）：

1. **close `stopChan`** — 通知消费 loop 退出。
2. **`loopWG.Wait()` / `wg.Wait()`** — 等待消费 loop 完全退出。这一步确保不会有新的 `dispatch` 调用，避免后续 `close(dispatchQueue)` 触发 "send on closed channel" panic。
3. **`shutdownWorkerPool()`**（幂等 `sync.Once` 保护）：
   - `close(dispatchStop)` — 解除 `dispatch` 的潜在阻塞（防止步骤 1-2 之间 loop 卡在 dispatch）。
   - `close(dispatchQueue)` — 通知 worker 排空后退出（`for job := range dispatchQueue` 自然结束）。
   - `dispatchWG.Wait()` — 等待所有 worker 退出。

> ⚠️ **顺序很关键**：必须先等消费 loop 退出，再 `close(dispatchQueue)`。否则 `dispatch` 可能与 `close(dispatchQueue)` 并发，触发 panic。

#### panic 恢复层次

| 层次 | 机制 | 触发场景 |
|------|------|---------|
| **per-handler `defer/recover`**（invokeHandlers 内） | `slog.Error` + `sentry.CaptureException`，**worker 继续运行** | handler 内 panic（最常见） |
| **GoSafe**（worker goroutine 启动） | `slog.Error` + `sentry.CaptureException`，**worker 退出** | workerLoop 自身 panic（理论不应发生，belt-and-suspenders） |
| **GoSafe**（消费 loop 启动） | 同上 | loop/subscribeLoop 自身 panic |

per-handler recover 是 worker pool 的核心：它确保 handler panic 不会污染 worker pool 容量，N 个 worker 始终保持稳定。GoSafe 作为兜底防御非 handler 路径的 panic。

#### 与 LogBus worker pool 的对比

| 维度 | PubSubBus | LogBus |
|------|-----------|--------|
| worker 用途 | 调用 Subscribe handler | 批量刷盘到 DB |
| 队列类型 | `chan dispatchJob` | LogBus 自管 priority bucket |
| 默认 worker 数 | 16 | 1（单 writer，避免 DB 写入竞争） |
| 关闭顺序 | stopChan → loopWG.Wait → shutdownWorkerPool | stopChan → drain → close |

---

## 五、配置说明

### 5.1 config.toml

```toml
[bus]
# driver = "redis"    # 集群模式：基于 Redis Pub/Sub，支持多节点广播
# driver = "memory"   # 单机模式：基于内存 channel，无需 Redis
# 不设置则根据 Redis.Enabled 自动选择（Redis 启用 -> redis，否则 -> memory）

[pubsub]
# dispatch worker pool 配置（Task 23）。
# 消费 loop 收到消息后，通过 dispatchQueue 投递给 N 个 worker 并行调用 handler，
# 替代原本的 per-event goroutine，避免高吞吐场景下 goroutine 数量爆炸。
#   - workers：worker 协程数。零值 = 默认 16。环境变量：NETYADMIN_PUBSUB_WORKERS
#   - queue_size：dispatch 队列缓冲容量。零值 = 默认 1024。环境变量：NETYADMIN_PUBSUB_QUEUE_SIZE
# 队列满时 dispatch 阻塞（backpressure），让消费 loop 反压到上游 Publish；
# 关闭时（Close）先停止消费 loop，再关闭 dispatchQueue，worker 排空后退出。
workers = 16
queue_size = 1024
```

### 5.2 内置订阅者注册

所有订阅者在 `internal/app/wire.go` 的 `Bootstrap` 函数中统一注册：

```go
// ConfigSync
safeSubscribe(eventBus, pubsub.TopicConfigSync, func(ctx context.Context, msg []byte) {
    _ = configWatcher.ForceReload(ctx)
})

// StorageSync
safeSubscribe(eventBus, pubsub.TopicStorageSync, func(ctx context.Context, msg []byte) {
    _ = services.storageConfig.LoadAllConfigs(ctx)
})

// CacheInvalidation — 仅失效本地 L1，避免递归
safeSubscribe(eventBus, pubsub.TopicCacheInvalidation, func(ctx context.Context, msg []byte) {
    var tags []string
    if err := json.Unmarshal(msg, &tags); err == nil {
        _ = cacheLifecycle.InvalidateL1ByTags(ctx, tags...)
    }
})

// IPACReload
safeSubscribe(eventBus, pubsub.TopicIPACReload, func(ctx context.Context, msg []byte) {
    _ = services.ipac.ReloadCache(ctx)
})
```

> **重要**：CacheInvalidation 的订阅者调用的是 `InvalidateL1ByTags` 而非 `InvalidateByTags`，因为后者内部会再次 Publish，会导致无限递归。

### 5.3 worker pool 调优建议

| 场景 | workers | queue_size | 说明 |
|------|--------|-----------|------|
| 单机开发 | 4-8 | 256 | 减少 goroutine 开销 |
| 单机生产 | 16（默认） | 1024（默认） | 兼顾吞吐与延迟 |
| 多机集群（高吞吐） | 32-64 | 2048-4096 | 视 handler 平均耗时与消息速率调整 |
| handler 耗时长（如全量 reload） | 32+ | 2048+ | 避免队列满导致 Publish 反压 |

> **调优经验**：
> - worker 数 ≈ CPU 核数 × 2 是常见起点，但若 handler 主要是 IO（如 DB / Redis），可大幅提高。
> - queue_size 应足以容纳"短时突发"（如配置变更触发的级联 cache invalidation），但不应过大以免掩盖背压问题。
> - 监控指标：`cap(dispatchQueue) - len(dispatchQueue)` 反映 worker 消费能力；持续接近 0 说明 worker 不够或 handler 太慢。

---

## 六、缓存失效的分布式同步机制

PubSubBus 承担了缓存模块的分布式失效同步职责。以下是完整的数据流：

### 6.1 失效广播流程

```text
Machine A                        Redis                         Machine B
─────────                        ─────                         ─────────
InvalidateByTags(tags)
  │
  ├── 1. 本地失效 L1 + L2
  │
  └── 2. eventBus.Publish(
           TopicCacheInvalidation,
           tags)
              │
              └──────────────────► system_bus ──────────────────►
                                                                   │
                                                          3. InvalidateL1ByTags(tags)
                                                             仅失效本地 L1
                                                             (L2 已由 A 清除)
```

### 6.2 关键设计决策

1. **仅失效 L1**：收到广播的机器只失效本地 L1 (BigCache)，不操作 L2 (Redis)。因为 L2 在发起失效的机器上已经被清除，其他机器直接回源 L2 即可拿到最新数据。

2. **避免递归**：`InvalidateByTags` 内部会调用 `eventBus.Publish`，因此订阅者必须调用 `InvalidateL1ByTags`（仅本地失效，不再广播），否则会形成无限递归。

3. **幂等性**：缓存失效是幂等操作，多次执行无副作用。

---

## 七、二次开发指南

### 7.1 新增 Topic

**步骤**：

1. 在 `internal/pkg/pubsub/topics.go` 中添加常量：

```go
const (
    TopicConfigSync        = "config_sync"
    TopicStorageSync       = "storage_sync"
    TopicCacheInvalidation = "cache_invalidation"
    TopicIPACReload        = "ipac_reload"
    TopicYourBusiness      = "your_business"  // 新增
)
```

2. 在发布者 Service 中注入 `pubsub.EventBus`，调用 `Publish`：

```go
type yourService struct {
    repo     YourRepository
    eventBus pubsub.EventBus
}

func (s *yourService) DoSomething(ctx context.Context) error {
    // 业务逻辑...

    _ = s.eventBus.Publish(ctx, pubsub.TopicYourBusiness, map[string]string{
        "action": "created",
        "id":     "123",
    })
    return nil
}
```

3. 在 `internal/app/wire.go` 的 `Bootstrap` 函数中注册订阅者：

```go
_ = eventBus.Subscribe(pubsub.TopicYourBusiness, func(msg []byte) {
    var payload struct {
        Action string `json:"action"`
        ID     string `json:"id"`
    }
    if err := json.Unmarshal(msg, &payload); err == nil {
        // 处理消息...
    }
})
```

4. 在 `initServices` 中将 `eventBus` 注入到 Service 构造函数。

### 7.2 注意事项

1. **严禁硬编码 Topic**：所有 Topic 必须在 `topics.go` 中注册，违反此规则会导致消息无法路由。

2. **Handler 中避免阻塞**：`dispatch` 方法为每个 Handler 启动独立协程，但仍应避免在 Handler 中执行耗时操作。如需处理耗时逻辑，应投递到任务队列。

3. **Handler 中避免递归 Publish**：如果 Handler 中需要调用 `InvalidateByTags` 等会触发 Publish 的方法，必须使用不会再次 Publish 的替代方法（如 `InvalidateL1ByTags`），否则会形成无限递归。

4. **消息无持久化**：PubSubBus 是"发后即忘"模式，不保证消息持久化。如果节点在发布时未订阅，消息会丢失。对于需要可靠投递的场景，应使用任务队列。

5. **MemoryDriver 的局限**：MemoryDriver 仅在当前进程内传播消息，不支持跨节点。如果部署多节点，必须使用 RedisDriver。

---

## 八、与 LogBus 的关系

| 维度 | PubSubBus | LogBus |
|------|-----------|--------|
| 数据流方向 | 从外向内（Redis → 应用） | 从内向外（应用 → 数据库） |
| 传输内容 | 配置变更、缓存失效、规则重载等通知 | 操作日志、错误日志等记录 |
| 消费模式 | 实时响应 | 批量缓冲写入 |
| 协程模型 | 1 个常驻订阅协程 | 1 个常驻写入协程 |

两者共同构成 NetyAdmin 的"常驻协程骨架"。

---

## 九、投递语义与重连兜底（Task 13）

### 9.1 At-Most-Once 投递语义

PubSubBus 基于 Redis Pub/Sub 实现，**不保证消息持久化与可靠投递**，属于 **at-most-once** 语义：

- **发布时订阅方未连接**：消息丢失（Redis Pub/Sub 不缓存历史消息）
- **订阅方断连期间发布的消息**：全部丢失（重连后不会补发）
- **发布时 Redis 不可用**：`Publish` 返回 error，调用方应 `slog.Error` 上报监控（Task 13.1）
- **订阅方 handler panic**：由 `GoSafe` 捕获 + Sentry 上报，不影响其他订阅者

**关键约束**：

1. **Publish 失败必须 slog.Error**：所有 `_ = bus.Publish(...)` 模式禁止，必须检查 error 并上报。cache 模块的 `InvalidateByTags` / `DeleteAndBroadcast` 已遵守此规范（Task 13.1）。
2. **Publish 失败仅日志不重试**：cache 失效场景下，本地 L1+L2 已清，跨节点 L1 失效漏掉由 TTL 兜底；不重试避免雪崩。
3. **不适用于需要可靠投递的场景**：如需 at-least-once 或 exactly-once，应使用任务队列（`pkg/task`）。

### 9.2 重连兜底机制（OnReconnect + L1 兜底）

针对「订阅方断连期间漏收 cache_invalidation 广播」的核心问题，PubSubBus 提供 `OnReconnect` 回调机制作为兜底：

```text
                    正常流程                          断连兜底流程
                    ────────                          ──────────

Machine A: InvalidateByTags(tags)           Machine A: InvalidateByTags(tags)
              │                                            │
              ▼                                            ▼
       bus.Publish(                           bus.Publish(
         TopicCacheInvalidation,                TopicCacheInvalidation,
         tags)                                  tags)
              │                                            │
   ┌──────────┴──────────┐              ┌──────────┴──────────┐
   │                     │              │                     │
   ▼ Redis               ▼ Redis        ▼ Redis               ▼ Redis (断连)
   正常投递               (断连)         正常投递               消息丢失 ✗
   │                     ✗              │
   ▼                     │              ▼
Machine B: 收到广播       Machine B:     Machine B:
   │                     漏收 ✗         收到广播
   ▼                                  (后续)
InvalidateL1ByTags      (L1 持有        │
                        过期数据)       ▼
                                    InvalidateL1ByTags

                                       断连恢复后：
                                       subscribeLoop → fireReconnect()
                                       → cacheLifecycle.reloadL1All()
                                       → no-op（仅告警，不清空 L1）
                                       → L1 stale 条目由 TTL 自然过期
                                       → L2 (source of truth) 重连后有效
                                       → FetchFast L2 命中回填 L1 ✓
```

**设计要点**：

1. **仅 RedisDriver 触发**：`MemoryDriver` 无重连概念，注册的 `OnReconnect` 回调永不调用（进程内 channel 不会断连）。
2. **首次连接不触发**：`subscribeLoop` 用 `hasDisconnected` 标志区分「首次连接」与「断连后重连」，避免应用启动时误触发 reconnect 回调（`reloadL1All` 现为 no-op，但回调仍只对真实断连恢复有意义）。
3. **回调在独立 goroutine 执行**：`fireReconnect` 内部用 `GoSafe` 包裹，不阻塞订阅循环；回调 panic 由 `GoSafe` 捕获 + Sentry 上报。
4. **reloadL1All 现为 no-op（P1-2 fix）**：原方案调用 `bigcache.Reset()` 清空 L1，但会引发 thundering herd（N 个不同 key 并发回源击穿 DB）。现改为 no-op（仅 `slog.Warn` 告警），L1 stale 条目由 TTL 自然过期，L2 (Redis) 重连后仍是 source of truth，`FetchFast` 在 L2 命中时回填 L1。详细设计决策见 [缓存模块 §5.1.3](./server-module-cache.md#513-pubsub-重连-l1-兜底设计p1-2-fix)。
5. **回调可选**：若未注册 `OnReconnect`，`fireReconnect` 是 no-op，`subscribeLoop` 正常运行。cache 模块在 `CacheLifecycle.SetEventBus` 时自动注册。

### 9.3 配置建议

| 场景 | bus driver | L1 | OnReconnect | 说明 |
|------|-----------|-----|-------------|------|
| 单机开发 | memory | 启用 | 不触发（无重连） | 进程内 channel 不会断连 |
| 单机生产 | memory | 启用 | 不触发 | 同上 |
| 多机集群 | redis | 启用 | 触发（no-op 告警） | **推荐**：断连兜底（L1 staleness 由 TTL 兜底） |
| 多机集群 | redis | 关闭 | 触发（no-op） | L1 关闭时 `reloadL1All` 无 L1 可清，无副作用 |

> **多机部署必须用 redis driver**：memory driver 仅在进程内传播消息，断连/重连概念不适用，多节点部署时缓存/IPAC/配置失效不会跨节点同步。
