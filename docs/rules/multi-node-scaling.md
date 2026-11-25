# 多机部署与水平扩展规则（V2）

> 目标：任何模块需要时都能水平扩展；不把"以后会瓶颈"留到上线后才发现。
> 规则面向 AI/开发者，默认 MUST；违反需在 PR 说明理由。
> 关联：状态策略 `contracts/state-policies.yaml`。

## 1. 总则

- 默认无状态；状态外置到 DB / Redis / 对象存储。
- 每个模块在 Manifest 声明 `scaling.mode`。
- 多机正确性优先：本地回退只在 `topology=single` 允许。
- 不做未发生的过度设计；但边界、幂等、超时、观测必须现在做对。

## 2. 模块伸缩模式

| mode | 含义 | 必须满足 |
|------|------|---------|
| `stateless` | 任意副本 | 无进程内正确性状态 |
| `leader` | 全集群单活 | 选主 + 租约续期 + 任务幂等 |
| `sharded` | 按 key 分片 | 分片键 + 重平衡 + 幂等 |
| `stateful` | 连接/会话绑定节点 | 连接归属 + 故障转移 + 排空 |

- Manifest 必填；CI 校验；新模块未声明 mode 拒绝合并。
- 不确定时选 `stateless`；有状态组件必须显式说明。

## 3. 无状态与状态外置

- 会话/令牌：JWT + Redis/DB；禁止进程内 session。
- 上传/附件：对象存储；禁止本地磁盘作为共享状态。
- 临时文件：请求级 temp；禁止共享本地 FS。
- 锁/计数/Nonce/幂等：coordination 平面；禁止 `sync.Mutex` 做跨节点正确性。
- 本地缓存：仅 cache 平面；generation 失效；禁止安全/协调数据进 L1。
- 黏性会话：非必需；仅 WS 可选。
- 进程内全局可变状态：禁止（只读配置、连接池、本地缓存除外）。

## 4. 数据边界

- 模块独占表；禁止跨模块 JOIN / 事务 / 外键。
- 跨模块只走 Port 或事件；批量读用批量 Port。
- 每模块可独立 datasource；读写分离可选。
- 查询必须分页 + 命中索引；禁止无界扫描。
- 迁移 expand-contract；向后兼容。
- 大表分区/归档由模块负责。

## 5. 服务间调用

- 只走 `api/` Port；local/remote 同一接口。
- Manifest 方法元数据：`idempotent/timeout/retryable/max_retries/idempotency_key`。
- 非幂等禁止重试；要求幂等的必须强制幂等键。
- 超时、熔断、隔离（bulkhead）；禁止无界 goroutine。
- 禁止 N+1；批量、缓存、异步。
- 服务 JWT + 端口级 AuthZ；不信任内网。

## 6. 有状态组件

| 组件 | 规则 |
|------|------|
| realtime | 本地 Hub；Redis 频道广播；`node_id` 路由；无需 sticky；连接/消息限流；关闭 1001；排空 |
| scheduler | cluster 任务选主；shard 分片；node 任务幂等；运行记录；故障切换 |
| L1 cache | 每节点独立；generation 广播；旧条目读取时淘汰 |
| LogBus | 每节点缓冲；关闭 flush；禁止共享文件 |
| 限流/Nonce/锁 | coordination 平面；multi 禁止 local |
| 选主 | Redis 租约或 PG advisory lock；续租；failover |

## 7. 容量与瓶颈预防

- 每模块声明 SLO：每副本 QPS、p99、内存、连接数。
- 发布前基准测试；CI 可选但必须留报告。
- 扩容触发：CPU > 70%、p99 > SLO、队列深度、连接数、错误率。
- 压测按峰值；同时观察 DB/Redis。
- 热路径预算：local < 1ms、RPC p99 < 10ms、DB p99 < 20ms、API p99 < 2000ms。
- 连接池：DB max = 副本数 × pool；Redis 池按副本。
- Redis：hash tag、无大 key、TTL、pipeline。
- 新模块 PR 必须写容量评估（QPS/存储/增长）。

## 8. 部署与滚动

- stateless 副本无状态；LB 轮询/least-conn。
- `maxUnavailable`、PDB、anti-affinity。
- readiness 检查必需依赖；可选缓存降级 ≠ 未就绪。
- 优雅关闭：停止接收 → drain → flush → close。
- 各副本配置一致（node_id 除外）：`deployment.node_id`（env `NETYADMIN_DEPLOYMENT_NODE_ID`，
  默认 `node-1`）——全集群唯一；日志 node_id/Bus consumer/scheduler 分片/GW 统计快照
  文件名同源（字符集 `[a-z0-9._-]`，非法拒启）。
- 滚动 N-1 兼容；禁止破坏式迁移。
- 金丝雀/蓝绿用于高风险变更。
- 每节点指标/日志/trace 带 `node_id/module`。
- 同机多进程验证形态（双实例共享 PG/Redis/日志目录）是 multi 的最小真机复现：
  node-1:18080 / node-2:18081；Bus Broadcast 跨节点收敛、outbox 双 relay 并行 claim、
  GW 统计每节点快照聚合均在此形态验证。

### 8.1 统计文件面边界

- 每进程只写本节点快照 `logs/<date>/gw_stats_hourly_<node>.jsonl`（文件名带 node_id
  ——同机多进程不再互相覆盖）；旧版单文件 `gw_stats_hourly.jsonl` 升级窗口内仍被读取。
- 管理面 stats 端点聚合读**本机日志目录下全部节点快照**（同机多进程形态可看全量历史）；
  内存实时面仅本节点当前小时。
- 跨机历史聚合=日志采集器（file shipping → 集中存储）职责；本进程只认本机目录。

## 9. 降级

- 禁用模块零开销：无路由/Job/goroutine/连接。
- Redis 故障：按 `state-policies`；multi 协调 fail-closed；cache 回源 DB。
- 禁止 multi 本地回退。
- 降级状态暴露：`/diagnostics/state`。

## 10. Review / CI

- [ ] Manifest `scaling.mode` 已声明。
- [ ] 无进程内正确性状态。
- [ ] 无跨模块 SQL/JOIN/TX。
- [ ] Port 方法元数据完整。
- [ ] SLO 与容量评估。
- [ ] 降级矩阵测试。
- [ ] 滚动/版本偏差（N-1）测试。
