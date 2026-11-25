# 更新日志

本文件记录 NetyAdmin 基座的对外能力变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added

- **admin 邮件设置（SMTP 运行期配置）**：系统管理 → 邮件设置页，读取/保存/重置 SMTP 参数并发送测试邮件；存储走 configcenter（版本/历史/回滚/操作审计复用），保存后经广播热更到各节点（重建 SMTP 驱动，无需重启）；启动期 `message.smtp` 为回退默认值；**密码恒为 env-only（`NETYADMIN_MESSAGE_SMTP_PASSWORD`），不入库不回显**；权限码 `admin.settings.smtp.view/operate` 随迁移种子交付。
- **admin 观测页**（运维 → 可观测）：指标概览（HTTP 5xx/QPS/p95、Bus/Scheduler 死信、Outbox 积压、Realtime 连接、Message 队列/待发/失败、状态模式）+ 阈值告警条 + 指标浏览器（含下游自注册 `na_*`）；权限码 `admin.observability.view` 随迁移种子交付，无外部 Prometheus/Grafana 依赖；`/metrics` 保持可被外部采集。
- `kernel.Metrics` 端口：`*observability.Metrics` 经 Registry Expose，模块与下游业务指标统一 `Register(collector)` 到共享 registry（与 `/metrics`、观测页同源）；配套 `Snapshot()`（族投影）、`Summarize()`（概览）、`Evaluate()`（阈值评估）。
- message 发送链路指标：`na_message_queue_depth`、`na_message_pending_records{channel}`（scrape 时查库，走 `(channel,status)` 索引）、`na_message_send_failed_total{channel}`。
- `config.observability.alerts`：观测页阈值覆盖（默认集见可观测契约 §6；未知键启动告警）。
- `message.Templates` 端口（模板自举）：下游模块可幂等声明自己的通知模板（`Ensure` 按 code 唯一、多副本冲突自动收敛、通道不符 fail-loud），配合 `message.Sender` 完成邮箱/短信验证码等链路，无需管理端预建模板；管理端对模板的编辑与禁用保持权威。
- `message/api` 错误哨兵与常量：`ErrTemplateNotFound/ErrTemplateDisabled/ErrTemplateSpecInvalid/ErrTemplateCodeInvalid/ErrTemplateConflict/ErrNoActiveDriver/ErrNotReady`（下游 `errors.Is` 判定，不依赖 domain 包、不匹配错误文本）、`TemplateStatusEnabled/Disabled`、`ChannelEmail/ChannelSms`。

### Fixed

- 模块开关与菜单一致性：`rbac_menus` 新增 `module` 列（空=常显），`route-tree` 按装配事实源（已启用模块）过滤——`users/message/storage` 关闭时对应菜单不再以死链接出现（此前默认 `users.enabled=false` 仍显示「C 端用户」）；空目录自动剪除；菜单管理页展示所属模块。
- 多机观测：全部指标（含下游 `Register` 的自定义 collector）注入 `node_id` ConstLabel（multi-node-scaling §8），`/metrics` 可按节点区分；观测页标明当前节点与数据作用域（「本节点」/「集群」），避免把单节点队列/失败数误当集群值。
- 观测面健壮性：请求时长 p95 落在 histogram 的 `+Inf` 桶或桶上界含 `+Inf` 时不再产生 `+Inf`（会被 JSON 拒绝导致端点空响应）——桶投影过滤非有限上界、p95 饱和为最大有限桶；`observability.alerts` 拒绝 NaN/Inf 阈值。
- identity 会话指标接线：此前 AttachDB 自建第二个 Metrics 实例，`na_session_*` 永远不出现在 `/metrics`/观测页；现使用共享实例。
- 观测页：自动刷新同时覆盖概览与指标浏览器（`v-loading` 不闪烁）；`na_message_pending_records` 采集带 15s 结果缓存（scrape 高频不放大 DB 压力）；阈值未知键改为启动期告警（与部署角色无关，`role=core` 也可见）。
- `message` 模板名称改为不可变业务键：管理端拒绝改名（409）——此前改名会让下游 `Ensure` 按原 code 静默新建第二行，破坏「按 code 唯一」契约；管理端编辑页名称栏在编辑态只读。
- `message` 模块：模板/发送记录「不存在」此前由适配器私有哨兵返回，与领域哨兵不匹配，管理面映射为内部错误；现按统一信封 404 返回（`errors.Is(domain.ErrTemplateNotFound/ErrRecordNotFound)` 生效）。模板更新撞唯一约束同样由 500 归正为 409。
- 配置生效性：`server.trusted_proxies`（反向代理 `X-Forwarded-For` 信任面）与 `log.compress`（轮转备份 gzip）此前解析后未接入运行组件，YAML 设值静默无效。现按配置生效；反代/负载均衡部署必须登记信任 CIDR，否则审计记录、登录限流与网关 IPAC/配额按代理 IP 判定（`deploy/config.example.yaml`、启动手册与安全基线已补说明）。

## [2.0.0] - 2026-11-22

首个正式版：可二次开发的 Go 管理后台基座（模块化单体）。

### Added

**基座与模块契约**

- 分层架构：`kernel ← platform/business ← edges` 单向依赖；模块经 Manifest / Port / Event / Job / Hook / Migration 契约装配，可按配置启用/停用。
- 模块脚手架：`node scripts/scaffold/new-module.mjs` 生成 Manifest、装配骨架、api 封装与分层目录。
- 部署形态：单机 all-in-one、按 role（all/core/admin）过滤接入面拆分、多副本水平扩展；模块声明 `scaling.mode`（stateless/leader/sharded/stateful）。
- 迁移体系：5 位号段、checksum 校验、schema/seed 分离、每 up 对称 down；下游使用独立号段与 `contracts/*.local.yaml` 登记，不修改基座契约。

**认证、权限与管理端**

- 管理员登录（英数验证码）、JWT 轮换与吊销（token_version）、首启随机密码、忘记密码重置命令。
- RBAC：角色 / 菜单 / API / 按钮四级权限，动态路由，策略热更广播；权限种子随模块迁移交付。
- Vue 3 + Vite + TS 管理端：登录、系统管理、开放平台、任务中心等页面；生产由后端同源服务。

**平台服务**

- 配置中心：动态键值、历史与回滚、Bus 热更广播。
- 审计：管理端操作写 `operation_logs`，可查询/保留/导出，默认 180 天保留清理。
- 存储：S3 兼容 presigned 直传 + HMAC 完整性防伪，支持 COS 等虚拟主机寻址服务。
- 消息：站内信 + 邮件（SMTP）/短信（log 驱动）+ 模板；驱动可替换。
- 实时推送：WebSocket 三面（admin/client/gateway），一次性 ticket 握手、频道两级 ACL、心跳/背压/限流全配置化。
- 开放平台网关：应用（AppKey + AES-256-GCM AppSecret）、scope 组、API 登记、IP 组集、配额、HMAC-SHA256 验签、进程内转发、调用统计、WS ticket 下发。

**内核设施**

- Cache：L1（进程内）+ L2（Redis），entity class 声明制，generation 跨节点失效，PII 实体 L2 加密，命中率/QPS 自动降级。
- Bus：Local / Broadcast / Cluster 三种投递语义 + 事务内发件箱（outbox）。
- 会话与协调：外置 JWT 会话、分布式锁、选主、幂等键、计数器、策略限流。
- 调度：cron/interval Job，集群选主去重，可选跨节点互斥与幂等窗口。
- 传输：统一信封、分页、绑定、权限中间件、六段请求 Hook、安全响应头、CORS、限流。
- 可观测：Prometheus 指标、OpenTelemetry 追踪（可选）、存活/就绪/诊断端点。
- 日志：`logs/<日期>/<module>_<type>.log` 文件化（app/access/error/security/audit），动态级别开关，脱敏。

**契约与文档**

- OpenAPI 3.1 三面契约（admin / client / gateway），由代码注解生成并在 CI 做破坏性 diff。
- 5 位状态码、命名空间、能力目录、Profile、状态策略、迁移号段的单一事实源与生成常量。
- 下游文档体系：下游入口索引、开发指南、AI 开发规则、网关签名契约、部署说明、API 契约说明。

**许可**

- Apache License 2.0。

### Changed

- 架构与约定变化：移除字典体系；统一 5 位字符串状态码与 `{code, message_key, data}` 信封；管理端统一路径前缀 `/admin/v1`；配置改为 YAML + 显式环境变量映射 + env-only 密钥；前端改为 Vue 3 + Element Plus。
- 运行期配置以 `config.yaml` 为基线，密钥（DB 密码、网关加密钥、SMTP 密码、存储盐、Cache L2 密钥）只经环境变量注入。
- 部署不使用容器编排：PostgreSQL/Redis/对象存储均为外部独立服务。

### Fixed

- 首个正式版，无对 2.0.0 自身的修复记录。

### Security

- 密钥分层：敏感项仅 env 注入；生产缺 JWT 密钥文件、占位 DB 密码、非法网关加密钥均拒绝启动（fail-closed）。
- 登录验证码一次性消费（Redis GETDEL，多节点一致）；登录失败与限流策略 fail-closed。
- 网关 AppSecret AES-256-GCM 存储，HMAC-SHA256 验签 + nonce 防重放 + 时间窗 + 配额 + IP 组集。
- WS ticket 单次使用、短 TTL、绑定 surface 与身份；订阅与消息两级 ACL。
- 安全响应头基线（nosniff / DENY / Referrer-Policy）恒发；HSTS 仅生产；CSP 默认 Report-Only，可配置强制。
- 日志与导出脱敏；运行/错误日志不写业务 DB。

### 升级说明

- v2.0.0 为首次发布，无历史版本升级动作；新部署按 [README 快速开始](README.md#快速开始5-分钟) 或 [deploy/README.md](deploy/README.md) 执行。
- 后续版本在此文件记录破坏性变更与升级步骤；发布 tag 即契约（`contracts/`）快照。
