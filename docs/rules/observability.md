# 可观测契约（observability）

> 日志路径/类型/开关见 logging.md；本文件定义三类记录的边界、指标/追踪契约与诊断端点约定（对应补齐清单「可观测契约」P0 项）。

## 1. 记录边界（一次产生，唯一归属）

| 记录 | 产生方 | 归属 | 存储 |
|------|--------|------|------|
| AccessRecord（HTTP/WS/RPC 访问） | transport | Audit | DB/文件 Sink |
| OperationRecord（管理端变更） | application | Audit | `operation_logs`（DB，保留 180 天） |
| SecurityRecord（登录失败/越权/限流） | security | Audit | DB |
| 技术日志（运行/错误） | 所有模块 | Observability | stdout/文件（**禁写 DB**） |
| 指标 / 追踪 | 所有模块 | Observability | Prometheus / OTel |

## 2. 日志 schema（结构化 JSON，字段与 logging.md 一致）

必带：`ts`、`level`、`module`、`type`（app/access/error/security/audit）、`msg`、`request_id`、`trace_id`、`node_id`、`source`。
级别使用：error=需要人处理；warn=自动恢复但需关注；info=业务关键路径；debug=默认关闭。

## 3. 指标契约

- 命名：`na_<subsystem>_<name>_<unit>`（如 `na_http_request_duration_seconds`）；命名空间冲突由 namespaces.md 治理。
- 标签**低基数**：`module/type/surface/code/method`；禁止 `id/request_id/user` 等高基数标签。
- 必备指标（最低集）：

| 子系统 | 指标 |
|--------|------|
| HTTP | 请求计数、时长直方图、在途数（按 surface/module/code） |
| RPC | 时长、错误计数、熔断状态 |
| DB | 查询时长、事务时长、错误计数 |
| Cache/Coordination | hit/miss、降级计数、失效延迟 |
| Bus | 发布/投递/失败计数、端到端延迟、DLQ 深度 |
| Scheduler | scheduled/started/succeeded/failed、时长、滞后、DLQ |
| Message | 队列深度、待发/可重试记录数、发送失败计数（按 channel） |
| WS | 连接数、消息计数、发送队列水位 |

- **自定义/下游指标**：模块与下游业务指标统一注册到共享 registry（端口 `kernel.Metrics`：
  `ResolveAs[*observability.Metrics]` → `Register(collector)`），命名沿用
  `na_<module>_<name>_<unit>`。同一实例同时服务 `/metrics` 抓取与 admin 观测页——
  **禁止另建 registry**（自建实例的指标不会被抓取/展示）。
- **admin 观测面**：`GET /admin/v1/system/observability/overview`（概览卡片 + 阈值告警）
  与 `GET .../observability/metrics`（指标族投影，family 前缀过滤 + 样本上限；
  含下游自注册指标），权限码 `admin.observability.view`。观测页是**展示型**：
  阈值在请求时评估，不落库/不推送；外部告警仍走 `/metrics` + Prometheus 规则。
  页面自动刷新（30s）同时覆盖概览与指标浏览器；`na_message_pending_records`
  采集带 15s 结果缓存（scrape/页面高频刷新不放大 DB 压力）。

- **多机数据作用域**（multi-node-scaling §8「每节点指标带 node_id」已满足）：
  指标经统一 registerer 注入 `node_id` ConstLabel（含下游 `Register` 的自定义
  collector），Prometheus 多节点抓取按 instance/node_id 区分。观测页是**本节点视图**：
  卡片标注「本节点」的为进程内值，「集群」的为数据库全局计数（Bus/Scheduler
  DLQ、Outbox、消息待发由各节点周期性从 DB 校准）；多节点聚合看 Prometheus，
  勿对全局 gauge 求和（取 max/last）。

## 4. 追踪契约

- W3C traceparent；传播链 HTTP → Port → RPC → Bus → Job 全覆盖；Bus 消息 Headers 带 trace_id。
- 采样：开发全采；生产按率配置（初始基线 10%，可配），错误请求强制采样。
- 跨进程断链（服务未接线）必须显式记录，不允许静默无 trace。

## 5. 健康与诊断

- `/health/live`（进程活）与 `/health/ready`（依赖就绪）口径分离；ready 不通过即摘流量。
- `/admin/v1/system/diagnostics/*`：七类诊断数据面——modules/bindings/hooks/bus topics/scheduler jobs/config/state（`state` 承接原「缓存」，v3.4 更名）。

## 6. 告警最小集（与 slo.md 联动）

错误率突增、读/写 API p99 越限、任一子系统进入 DEGRADED、DLQ > 0、leader 缺失、熔断打开、备份失败。

内置阈值（admin 观测页；`config.observability.alerts` 可覆盖，未知键启动告警）：

| 键 | 默认 | 语义 |
|----|------|------|
| `http_5xx_rate` | 0.05 | 5xx / 总请求（窗口有流量时用窗口差分，否则累计） |
| `bus_dlq_depth` | 0 | >0 即 critical |
| `scheduler_dlq_depth` | 0 | >0 即 critical |
| `bus_outbox_pending_depth` | 1000 | 待发布积压 |
| `message_queue_depth` | 48 | 进程内队列（容量 64 的 75%） |
| `message_pending_records` | 1000 | 待发送/可重试记录数 |
