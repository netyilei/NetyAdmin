# 📝 NetyAdmin 统一日志聚合缓冲系统设计文档

## 1. 设计背景
目前系统中存在多种日志（操作日志、错误日志、任务日志、开放平台日志）。每种日志目前的处理方式不一，部分为同步写库，部分为简单的异步写库。在高并发场景下，频繁的数据库 I/O 插入会导致数据库连接池耗尽和磁盘 I/O 瓶颈。

## 2. 核心架构设计

### 2.1 模块定位
新建 `internal/pkg/logbus` 模块。它作为一个“中心化”的异步聚合器，负责接收全系统的日志条目，并根据配置策略执行批量写入。

### 2.2 核心接口定义
所有日志实体需实现 `LogEntry` 接口，以便 `LogBus` 识别存储目标。

```go
// internal/pkg/logbus/types.go
type LogEntry interface {
    TableName() string // 目标表名，用于批量写入时的路由
}
```

### 2.3 逻辑架构图
```text
[业务模块] -> Record(Entry) -> [LogBus Channel]
                                     |
                                     v
                          [Dispatcher (常驻协程)]
                                     |
                    +----------------+----------------+
                    |                |                |
             [Buffer: Ops]    [Buffer: Open]   [Buffer: Error]
                    |                |                |
                    +--------+-------+-------+--------+
                             | 触发条件(条数/时间)
                             v
                    [Batch Repository (DB)]
```

---

## 3. 详细设计方案

### 3.1 缓冲策略
- **内存分桶**：`LogBus` 内部维护一个 `map[string][]LogEntry`，按表名对日志进行归类。
- **双触发机制**：
    - **Size Threshold**：单类日志积压超过 `N` 条（默认 200）。
    - **Time Threshold**：距离上次写入超过 `T` 秒（默认 5 秒）。
- **背压控制**：设置全局 `chan` 容量。若积压过快，策略可选：
    - `DropOldest`：丢弃最旧日志（适用于高频调用日志）。
    - `SyncWrite`：转为同步写（适用于错误日志）。

### 3.2 存储引擎适配
- **GORM 优化**：使用 `db.CreateInBatches(entries, batchSize)`。
- **自动迁移兼容**：利用 GORM 的 `Schema` 自动识别实体字段，无需手动写 SQL。

---

## 4. 现有日志并入修改点

为了实现平滑迁移，建议对现有模块进行如下调整：

| 日志类型 | 现状 | 修改建议 | 风险/收益 |
| :--- | :--- | :--- | :--- |
| **开放平台日志** | 异步 + 简单缓冲 | 接入 `LogBus`，移除 Service 内部的 `processLogs` 逻辑。 | **收益**：代码更精简，统一管控内存。 |
| **操作日志** | 同步写入 (Middleware) | 改为调用 `LogBus.Record`。 | **收益**：消除写库对管理后台操作的延迟。 |
| **错误日志** | 异步写入 (Middleware) | 接入 `LogBus`，但设置较低的 `Time Threshold` (如 1s)。 | **风险**：系统崩溃时可能丢失极少量错误记录。 |
| **任务日志** | 异步回调写入 | 接入 `LogBus`。 | **收益**：大量定时任务并行时，减少对 DB 的瞬时冲击。 |
| **上传记录** | 业务 Service 实时写入 | **条件接入**：若仅作为审计日志，建议接入；若作为业务强关联数据，建议保留实时写入或设置极短间隔（如 <1s）。 | **风险**：异步延迟可能导致前端上传后立即查询时出现“记录不存在”。 |
| **消息发送流水** | 业务 Service 实时写入 | **部分接入**：仅将“发送状态更新”并入 `LogBus`，初始 Pending 记录保持实时写入以配合异步任务。 | **收益**：降低高频发信时的数据库写竞争。 |

---

## 5. 未来风险规避与扩展

### 5.1 存储介质“零成本”切换
未来如果单机 PostgreSQL 无法支撑日志量（如日活过千万），`LogBus` 的底层实现可以从 `GORM` 切换为 `Kafka` 或 `ClickHouse` 驱动，而业务层的 `Record(Entry)` 调用完全不需要改动。

### 5.2 优雅停机保证
在 `LogBus.Stop()` 时，必须遍历所有 `map` 中的分桶，将剩余数据强行执行最后一次 `BatchCreate`。

### 5.3 监控与告警
`LogBus` 可以暴露 Prometheus 指标：
- `logbus_dropped_total`：由于缓冲区满而被丢弃的日志数。
- `logbus_flush_latency`：数据库批量插入的耗时。

---

## 6. 后续实施建议

1.  **第一步**：在 `internal/pkg/logbus` 实现核心逻辑。
2.  **第二步**：修改 `internal/app/app.go` 初始化 `LogBus` 并注册到全局。
3.  **第三步**：逐步替换 `OperationLogService` 和 `OpenLogService` 的写入逻辑。
