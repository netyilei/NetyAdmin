// Package bus 是全局总线：local/broadcast/cluster 三种发布域、类型化消息信封
// （ID/Topic/Kind/SchemaVer/Payload/Headers/OccurredAt）、Outbox 事务一致性、
// Inbox 去重、DLQ 与重放、事件目录、trace_id 贯穿。Topic 命名见 `namespaces.md`
// Redis 键遵循 namespaces.md §4（na:{env}:bus:ch|q|inbox:...，消费组
// bus:default:cg），设施表迁移在 migrations/（core 段 bus 子段 11330+）。
//
// 交付（facade + 三域传输）：
//   - Local：进程内同步执行（per-handler 超时，错误返回发布方）。
//   - Broadcast：本地异步 + Redis Pub/Sub relay 每节点（at-most-once，有界队列
//     满丢弃计数；WithRemoteOnly 跳过本地段；本地模式 RemoteOnly 显式报错；
//     per-publisher 序列号 + 接收侧缺口检测 → na_bus_broadcast_lost_total，
//     §5.4 计数半项）。
//   - Cluster：Redis Streams 消费组竞争消费一次（at-least-once，PEL 回收重投；
//     per-topic 流、订阅即建组 MKSTREAM）。
//   - 单机无 Redis：Broadcast/Cluster 降级本地投递（仅 topology=single 允许，
//     AGENTS §3）；multi 无 Redis 构造报错。
//
// 交付（事务一致性 + DLQ + 目录）：
//   - Outbox：PublishToOutbox/PublishAfterCommit 与业务同事务写入（要求
//     database.WithTx；回滚即丢弃）；Relay 循环 FOR UPDATE SKIP LOCKED 批量
//     claim（ORDER BY id 保序）→ Cluster 域发布 → 标记 → 保留窗口清理。
//   - Inbox：Redis SET NX EX 消费去重原语（Acquire；TTL 到期可重处理；Redis
//     故障返回错误，调用方跳过处理保持 at-least-once）。
//   - DLQ：达到 MaxAttempts/毒消息入库留存（payload/headers TEXT 无损存原文）+
//     XAck 移出 PEL；ReplayDLQ 重建信封重发布；na_bus_dlq_depth 深度指标
//     （relay 每轮校准）。无 DB 时退回丢弃计数路径。
//   - 事件目录：module.Registry.EventsSnapshot（Manifest publishes/subscribes），
//     诊断面 events 数据源。SchemaVer 随信封贯穿；只加字段/语义变更升版本是
//     发布方纪律，双发窗口无内核代码。
//
// 域语义声明：域由发布方选择（WithDomain）——传输选择（Local/PubSub/Streams）
// 本质发生在发布侧；SubscribeOption（WithFilter/WithPriority/WithConcurrency/
// WithBlocking）在域语义之上叠加，分发路径（deliver）三域共用=语义一致。
//
// 规格能力的延后登记（outbox per-aggregate 全序 claim）统一在
// 带触发点），此处不复述。
package bus
