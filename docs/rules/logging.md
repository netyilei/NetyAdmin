# 日志规范（V2）

> admin 操作日志（审计）→ DB；运行日志、错误日志 → 磁盘文本/采集器，**不入 DB**。
> 关系：全局 G2「记录运行日志」；本项目选型见 §9。

## 1. 结论

- **组件**：Go 标准库 `log/slog` + `natefinch/lumberjack`（轮转）+ 小型 `log.Manager`（按模块/类型路由）。
- **不引入**：logrus、重型日志框架、自研日志框架、运行/错误日志入 DB。
- **备选**：`zap` / `zerolog` 仅在压测证明 slog 不足时替换 Handler；业务 API 保持 `slog`。
- **生产采集**：stdout → Vector / Fluent Bit / OTel Collector → Loki/ELK；文件作为本地与兜底。
- **admin 操作日志**：audit service 写 DB（`operation_logs`），用于查询/保留/导出。
- **运行/错误日志**：文本文件/采集器；错误日志不建 `error_logs` 表。

## 2. 为什么运行/错误日志不写 DB

- 高频写放大：锁、IO、表膨胀、索引维护、备份成本。
- 日志是 append-only 事件流，适合文件/采集器；DB 适合低频、可查询、合规审计记录。
- **admin 操作日志是审计记录**：低频、需查询/保留/合规，写 DB 合理；运行/错误日志是高频事件流，写磁盘/采集器。
- V1 教训：operation 日志可留 DB（作为审计），error 日志不应入 DB；V2 分离。

## 3. 日志类型（固定，不随模块膨胀）

| 类型 | 内容 | 去向 |
|------|------|------|
| `app` | 模块运行日志 | 文件/采集器 |
| `access` | HTTP/WS/RPC 访问 | 文件/采集器（可采样） |
| `error` | 错误/panic | 文件（+ Sentry/OTel 可选） |
| `security` | 认证/权限/限流/风控 | 文件（+ 审计按需） |
| `audit` | admin 操作日志（审计） | audit service DB（`operation_logs`；可文件镜像） |
| `stats` | 统计快照（聚合台账 gw_stats_hourly_<node>.jsonl——每节点独立文件；旧版单文件 gw_stats_hourly.jsonl 升级窗口内仍被读取） | 文件直写（不经 Manager 路由——JSON 行格式独立于全局 format；随目录清扫） |

- **debug 是级别，不是类型**。
- 新增类型需评审（G1-15）；禁止每模块自建类型。
- **访问流水全文件化**：HTTP/WS/网关访问日志、错误日志、调试日志一律写文件不进 DB（DB 仅承载审计合规面：管理端操作审计/authz deny/限流触发/登录失败/RPC 服务调用审计）； 统计=流水文件 gw_access.log + 每节点聚合快照 gw_stats_hourly_<node>.jsonl（零 DB；管理面聚合读本机全部节点快照，跨机历史=日志采集器职责）。

### 3.1 审计：admin 操作日志 → DB

- 表：`operation_logs`（保留 V1 表名，避免无谓迁移）。
- 字段：`actor, action, resource, method, path, status, biz_code, ip, ua, request_id, trace_id, detail(脱敏), created_at`。
- 写入：audit service 异步批量；关键安全操作可同步；失败写错误日志 + 指标。
- 查询/保留/导出；追加写、不可篡改；保留期按合规。
- 禁止记录请求体明文/PII/密钥；`detail` 脱敏。
- 运行/错误日志不得写入该表。

## 4. 文件路径

```
<log.dir>/<YYYY-MM-DD>/<module>_<type>.log
```

- 默认 `log.dir: ./logs`；在 `server/` 目录运行时即 `server/logs26-11-11/identity_app.log`。
- 按天目录；文件按 `module_type`。错误日志示例：`server/logs26-11-11/identity_error.log`。
- `audit` 不写文本文件（除非本地镜像）；admin 操作日志写 DB `operation_logs`。
- 单文件 `max_size_mb`；按天 + 大小轮转；保留 `retention_days`；可选压缩。
- 权限 `0640`；禁止 world-readable。

## 5. 配置

```yaml
log:
  level: info                 # debug|info|warn|error
  console: true               # 开发 true；生产 stdout
  format: json                # 开发 text；生产 json
  dir: ./logs
  retention_days: 30
  max_size_mb: 100
  max_backups: 10
  compress: true
  debug_ttl: 30m              # 可选：debug 自动恢复，防忘关
  modules:
    identity: { level: debug, types: [app, error, access] }
  types:
    app: true                  # 文本文件
    access: true               # 文本文件
    error: true                # 文本文件（主）
    security: true             # 文本文件
    audit: true                # admin 操作日志 → DB（operation_logs）
```

- 未列出的模块/类型按 `types` 默认；`modules` 可覆盖 level 与类型。
- 动态生效：configcenter 热更 + `slog.LevelVar` + 原子开关；无需重启。
- 生产禁止全局长期 debug；`debug_ttl` 到点自动恢复覆盖前级别（无覆盖回落全局级；reset/改非 debug 级即取消计时）。
- 实际默认（与样例值的差异）：`max_size_mb` 100 / `retention_days` 30 / `max_backups` 0=不限（按日目录组织）/ `compress` 默认关（样例 true 为示例值）；非法 `format` 启动 fail-fast。
- `retention_days` 同时驱动**按日目录清扫**：超过保留期的 `logs/<日期>/` 目录在启动与日期翻转时删除（lumberjack 只清理当前文件所在目录，历史日期目录由 Manager 补扫）。

## 6. 设计

- `log.Manager`：按 `(module, type)` 创建/缓存 `slog.Logger`；持有 sinks、level、switches。
- `Handler`：实现 `slog.Handler`；`Enabled` 检查全局 level + 模块/类型开关；`WithAttrs/WithGroup` 透传。
- `Sink`：console / file / OTel / Sentry；file sink = `lumberjack` + 日期目录包装。
- 字段：`ts, level, module, type, msg, request_id, trace_id, node_id, surface, principal, source, error, duration_ms`。
- `AddSource` 仅 debug/error 开启，避免常态成本。
- 脱敏：复用 `mask`/PII；禁止 raw body、密钥、完整证件号。
- 非阻塞：有界队列；debug/access 满则丢弃并计数；error/security 不丢（同步/阻塞）。
- 采样：access 高 QPS 可按路由/状态码采样。

## 7. 调试流程

1. 开发：`console: true`、`format: text`、`level: debug`。
2. 定位：把目标模块 `level` 调为 `debug`（配置/诊断端点），在关键分支加 `slog.Debug("...", "request_id", ...)`。
3. 控制台/文件即时可见；带 `request_id/trace_id/source` 定位。
4. 定位后恢复级别；`debug_ttl` 兜底。
5. 禁止用 DB 当调试输出。

## 8. 禁止

- ❌ 运行日志/错误日志写业务 DB；禁止建 `error_logs` 表。
- ❌ admin 操作日志只写文件不入 DB（审计必须 DB）。
- ❌ 自研日志框架 / logrus。
- ❌ 每模块自建文件/logger；统一走 `log.Manager`。
- ❌ 每个日志点一个开关；开关按类型/模块，不按行。
- ❌ 明文 PII/密钥/完整请求体。
- ❌ 无界缓冲/阻塞主路径。
- ❌ 永久 debug。

## 9. 开源组件对比

| 组件 | 优点 | 缺点 | 结论 |
|------|------|------|------|
| `log/slog`（stdlib） | 标准库、结构化、级别、Handler、`LevelVar`、`AddSource` | 无轮转/路由，需要小封装 | **默认** |
| `uber-go/zap` | 高性能、成熟、采样/Hook | 配置复杂、依赖更重 | 压测不足时可选 |
| `rs/zerolog` | 零分配、快、API 简单 | 生态较窄 | 可选 |
| `sirupsen/logrus` | 老牌、上手快 | 维护模式 | 不用 |
| `natefinch/lumberjack` | 轮转成熟（大小/备份/压缩） | 无日期目录 | **配合日期包装** |
| `lestrrat-go/file-rotatelogs` | 时间/日期模式、保留 | 无大小轮转、额外依赖 | 可选替代 |
| Vector / Fluent Bit / OTel Collector | 采集/转发/集中 | 运维组件 | 生产推荐 |

## 10. 验收

- 控制台可 debug；开关动态生效；debug TTL 生效。
- 文件路径符合 `logs/<date>/<module>_<type>.log`；轮转/保留生效。
- 运行/错误日志零 DB 写入；admin 操作日志入 `operation_logs`；无 `error_logs` 表。
- 脱敏、非阻塞、采样、字段 schema 有测试。
