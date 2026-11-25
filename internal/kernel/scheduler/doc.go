// Package scheduler 是全局调度器：cron/interval/once/delay/manual 触发、
// node/cluster/shard 执行域、Redis 租约选主（coordination）、单飞行
// 防重入、指数退避重试、运行记录（scheduler_runs，先写 running 再执行）、
// 优雅排空。Job 必须幂等（不承诺 exactly-once）。
//
// 交付（引擎）：
//   - 触发引擎 memory 驱动：robfig/cron per-tz 实例 + ticker/timer；durable
//     驱动（River，需法务确认 MPL-2.0）为可选演进。
//   - 执行域：node 每节点；cluster 仅 leader（Coord 空=自任 leader，仅单机
//     降级合法）；shard 按 hash(nodeID) 静态分片（动态再平衡随成员感知）。
//   - 跨节点守卫（机制面，默认关）：Spec.LockEnabled=coordination 互斥锁
//     （node 域跨节点重叠敏感任务，拿不到=skip+计数）；Spec.IdempotencyTTL=
//     coordination 幂等键（窗口内至多一次成功执行，失败释放可重试）。
//   - 多节点 liveness：每节点 Redis 心跳标记（TTL 15s/续 5s）；leader 周期
//     扫荡他节点僵尸 running 行（存活标记过期 → unknown）。
//   - 重试：指数退避+±20% jitter，Retries 上限后标记 failed——运行记录即
//     死信，TriggerNow 手动重放（不建独立 DLQ 表）。
//   - 运行记录：先写 running 再执行；启动时本节点 running→unknown（崩溃恢复，
//     多节点存活判定随）；排空超时在途行标记 unknown。
//
// 交付（诊断/指标/管理面内核侧）：na_scheduler_* 指标族（ticks/
// started/succeeded/failed/retrying/skipped/duration/lag/dlq_depth）；
// Catalog/RecentRuns 数据面；TriggerNow/Pause/Resume 管理面 API。
// taskcenter/adminapi 端点与运行时改 Spec 随。
//
// Job 契约：必须幂等且必须响应 ctx 取消（超时后执行 goroutine 存活至 Run
// 自返；重试前等待僵尸退出，无视 ctx 者阻塞至自返）。
// DST：时区含夏令时必须显式 DSTAware=true（fail-closed；歧义时刻墙钟行为
// 由使用方验收）。
// 登记延后：event/queue 触发（随消费方）、
// MissedRun run_once（随 durable 驱动）、DSTAware 歧义去重、Job 双源校验
// （Manifest 声明 vs 注册实例，首个模块 Job 接入时）。
package scheduler
