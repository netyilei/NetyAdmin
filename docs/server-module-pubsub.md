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
[PubSubBus (1 个常驻协程)]
      |
      +--- topic: config_sync --------> [ConfigWatcher.ForceReload()]
      |
      +--- topic: storage_sync -------> [StorageConfigService.LoadAllConfigs()]
      |
      +--- topic: cache_invalidation -> [LazyCacheManager.InvalidateL1ByTags()]
      |
      +--- topic: ipac_reload --------> [IPACService.ReloadCache()]
```

### 3.2 EventBus 接口

```go
type EventBus interface {
    Publish(ctx context.Context, topic string, msg interface{}) error
    Subscribe(topic string, handler func(msg []byte)) error
    Close() error
}
```

| 方法 | 说明 |
|------|------|
| `Publish` | 向指定 Topic 发布消息，msg 会被序列化为 JSON |
| `Subscribe` | 订阅指定 Topic，handler 接收的是消息的 Payload 原始字节 |
| `Close` | 关闭总线，停止订阅协程并释放资源 |

### 3.3 消息协议

所有消息通过统一频道传输，消息体结构：

```json
{
    "topic": "config_sync",
    "payload": "<JSON 编码的业务数据>",
    "timestamp": 1713715200
}
```

- `topic`：消息主题，用于路由到对应的订阅者
- `payload`：业务数据（`json.RawMessage`），由发布者序列化、订阅者反序列化
- `timestamp`：Unix 时间戳

### 3.4 Topic 注册表

**强制规范**：严禁在业务代码中硬编码 Topic 字符串，必须在 `internal/pkg/pubsub/topics.go` 中统一定义。

当前注册的 Topic：

| Topic 常量 | 值 | 发布者 | 订阅者 | 说明 |
|------------|-----|--------|--------|------|
| `TopicConfigSync` | `"config_sync"` | ConfigService | ConfigWatcher | 系统配置变更广播 |
| `TopicStorageSync` | `"storage_sync"` | StorageConfigService | StorageConfigService | 存储配置变更广播 |
| `TopicCacheInvalidation` | `"cache_invalidation"` | LazyCacheManager | LazyCacheManager | 分布式缓存失效同步 |
| `TopicIPACReload` | `"ipac_reload"` | IPACService | IPACService | IP 规则重载通知 |

---

## 四、驱动实现

### 4.1 MemoryDriver（单机模式）

基于 Go 原生 `channel` 和 `sync.Map` 实现进程内事件分发。

```text
Publish ──► msgChan (buffered 1000) ──► loop() ──► dispatch(topic, payload)
                                                    │
                                                    ├── handler1(msg)
                                                    ├── handler2(msg)
                                                    └── ...
```

**特点**：
- 无需部署 Redis，极轻量
- 消息仅在当前进程内传播，不支持跨节点
- 适合开发环境或单机部署

### 4.2 RedisDriver（集群模式）

基于 Redis Pub/Sub 实现，订阅统一频道 `{prefix}:channel:system_bus`。

```text
Publish ──► redisClient.Publish(channel, message)
                         │
                    Redis Server
                         │
                         v
              subscribeLoop() ──► dispatch(topic, payload)
                                       │
                                       ├── handler1(msg)
                                       ├── handler2(msg)
                                       └── ...
```

**特点**：
- 支持多节点间的广播通信
- 保证分布式缓存和配置的一致性
- 仅占用 1 个 Redis 连接

### 4.3 驱动选择策略

通过 `config.toml` 的 `[bus] driver` 配置控制：

| 配置值 | 行为 |
|--------|------|
| `"redis"` | 强制使用 RedisDriver，若 Redis 未启用则启动报错 |
| `"memory"` | 强制使用 MemoryDriver，即使 Redis 可用也不使用 |
| 不设置（默认） | 根据 `redis.enabled` 自动选择：Redis 启用 → RedisDriver，否则 → MemoryDriver |

---

## 五、配置说明

### 5.1 config.toml

```toml
[bus]
# driver = "redis"    # 集群模式：基于 Redis Pub/Sub，支持多节点广播
# driver = "memory"   # 单机模式：基于内存 channel，无需 Redis
# 不设置则根据 Redis.Enabled 自动选择（Redis 启用 -> redis，否则 -> memory）
```

### 5.2 内置订阅者注册

所有订阅者在 `internal/app/wire.go` 的 `Bootstrap` 函数中统一注册：

```go
// ConfigSync
_ = eventBus.Subscribe(pubsub.TopicConfigSync, func(msg []byte) {
    _ = configWatcher.ForceReload(context.Background())
})

// StorageSync
_ = eventBus.Subscribe(pubsub.TopicStorageSync, func(msg []byte) {
    _ = services.storageConfig.LoadAllConfigs(context.Background())
})

// CacheInvalidation — 仅失效本地 L1，避免递归
_ = eventBus.Subscribe(pubsub.TopicCacheInvalidation, func(msg []byte) {
    var tags []string
    if err := json.Unmarshal(msg, &tags); err == nil {
        _ = lazyCacheMgr.InvalidateL1ByTags(context.Background(), tags...)
    }
})

// IPACReload
_ = eventBus.Subscribe(pubsub.TopicIPACReload, func(msg []byte) {
    _ = services.ipac.ReloadCache(context.Background())
})
```

> **重要**：CacheInvalidation 的订阅者调用的是 `InvalidateL1ByTags` 而非 `InvalidateByTags`，因为后者内部会再次 Publish，会导致无限递归。

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
