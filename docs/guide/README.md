# 下游文档入口

> 读者：拿 NetyAdmin 做基座、开发自己业务的下游团队（后端 / 前端 / 第三方接入方）。
> 用法：按「我要做什么」查下表，每行直达具体文档。所有命令与默认值以 v2.0.0 为准。

## 按任务导航

### 上手与开发

| 我要… | 看这个 | 说明 |
|-------|--------|------|
| **第一次做模块（新手从这里开始）** | **[新手教程：从零做一个完整业务模块](tutorial-first-module.md)** | product 模块 Step 0–9：脚手架 → Manifest → 迁移 → 分层 → HTTP → 权限 → 装配 → 跑通 |
| **启动与运行（完整命令手册）** | **[启动与运行完整手册](getting-started.md)** | 全部参数 / 环境变量 / 配置文件逐段说明；源码与二进制两种启动路径；systemd；排错手册 |
| **写客户端对接 API 与 WS** | **[客户端接入指南](client-integration.md)** | 三面选择（admin/client/gateway）、HTTP 信封与认证、WS 五步接入（ticket→hello→订阅→心跳→重连）、可运行 Node 示例、排错 |
| 5 分钟把项目跑起来 | [项目 README · 快速开始](../../README.md#快速开始5-分钟) | 建库 → 配置 → 迁移 → 启动 → 登录 |
| 走一遍完整开发路径 | [下游开发指南](downstream-development.md) | 建模块 → 用基座能力 → HTTP/网关/WS 全流程 |
| 创建业务模块 | [下游开发指南 · 一创建模块](downstream-development.md#一-创建模块) | scaffold、Manifest、迁移号段、api/keys、装配登记 |
| 使用基座内核能力 | [下游开发指南 · 二使用基座功能](downstream-development.md#二-使用基座功能) | Cache / Bus / Session·Coordination / Scheduler / Hooks / 平台服务端口（通知/上传/审计/配置） |
| 发邮箱/短信验证码（不预建模板） | [下游开发指南 · 模板自举与验证码发送](downstream-development.md#模板自举与验证码发送完整链路) | `message.Templates.Ensure` 幂等建模板 + `message.Sender.Send`；无需管理端预建、不直写私有表 |
| 开发 HTTP API | [下游开发指南 · 三 HTTP API 开发](downstream-development.md#三-http-api-开发) | 路由惯例、统一信封、分页、权限码、OpenAPI 注解 |
| 把自己的 API 开放给第三方 | [下游开发指南 · 四加入网关](downstream-development.md#四-加入网关把你的-api-开放给第三方) | 治理面登记、HMAC 签名、失败码 |
| 写 WebSocket 客户端 | [下游开发指南 · 五 WS 开发](downstream-development.md#五-ws-开发) | 三面差异、ticket 流程、可运行客户端示例 |
| 让 AI 按基座契约写业务 | [下游开发提示词包（AI 用）](downstream-development-rules.md) | 把该文件 + 你的项目参数交给 AI 编码助手（Claude Code / Cursor 等）即可开始；含边界、硬规则、执行清单、项目 AGENTS 模板 |
| 开发管理端前端 | [admin/README.md](../../admin/README.md) | 技术栈、目录、WS 客户端复用、构建命令 |

### 契约与集成

| 我要… | 看这个 | 说明 |
|-------|--------|------|
| 读 HTTP API 契约 / 生成客户端 | [API 契约说明](../../contracts/openapi/README.md) | admin/client/gateway 三面 OpenAPI spec、生成与对接 |
| 用任意语言实现网关签名 | [网关 HMAC 签名契约](gateway-canonical.md) | canonical 串、请求头、时间窗、失败码 |
| 查状态码与含义 | [状态码规范](../rules/status-codes.md) | 5 位码分配、HTTP 映射、18xxx 网关族 |
| 查 WS 协议细节 | [WS 协议规范](../rules/ws-protocol.md) | 帧格式、频道、关闭码、限制、多节点语义 |
| 查命名空间 / key / 频道规则 | [命名规范](../rules/namespaces.md) | 前缀、owner、单一事实源、CI 校验 |
| 选能力 / Profile | [能力选择规范](../rules/capabilities.md) | 能力目录、lean/standard/full 预设、禁用语义 |

### 部署与运维

| 我要… | 看这个 | 说明 |
|-------|--------|------|
| 部署上线 | [deploy/README.md](../../deploy/README.md) | 配置样例、单机/拆分/多机形态、迁移与升级 |
| 多机与水平扩展 | [多机部署与水平扩展](../rules/multi-node-scaling.md) | 伸缩模式、状态外置、容量与滚动发布 |
| 热更与回滚 | [多机热更](../rules/hot-update.md) | 配置/能力/代码/schema 的边界与收敛 |
| 日志与审计落地 | [日志规范](../rules/logging.md) | 文件日志 vs 审计入库、级别、脱敏 |
| 安全基线 | [安全基线](../rules/security.md) | 密钥、认证授权、PII、传输 |
| 迁移规范（下游号段） | [数据库迁移规范](../rules/migrations.md) | 5 位号段、schema/seed 分离、expand-contract |
| 测试与 CI 门禁 | [测试规范](../rules/testing.md) | 测试库隔离、串行、契约测试、门禁 |
| 版本与升级承诺 | [版本与发布流程](../rules/release.md) · [CHANGELOG](../../CHANGELOG.md) | 六版本轴、弃用三步、下游升级 |

### 理解基座

| 我要… | 看这个 | 说明 |
|-------|--------|------|
| 读架构设计 | 本目录指南体系（`downstream-development.md` 分层/模块契约/状态平面/部署形态章节） | 设计说明以内嵌章节为准 |
| 查设计约束与非目标 | README「适用项目类型」+ `downstream-development-rules.md` §1 基座边界 | 选型与取舍口径 |
| 查全部开发规范 | [规则总入口](../rules/README.md) | 状态码/缓存/迁移/日志/API 风格等 MUST/MUST NOT |

## 排障速查

| 现象 | 先查 |
|------|------|
| 启动失败：`config: read file` | `config.yaml` 不存在 —— `cp deploy/config.example.yaml config.yaml` |
| 启动失败：`NETYADMIN_GW_SECRET_KEY` / GW key | 环境变量缺失或不是 base64 32 字节；`openssl rand -base64 32` 重新生成 |
| 启动失败：multi 拓扑无 Redis | `deployment.topology=multi` 必须配置 `redis.addr` |
| 启动失败：`config section ... not present` | `config.yaml` 缺少对应模块段（空段 `{}` 即可） |
| 启动日志 `config warning: 未知配置段` | 模块段（identity/rbac/gw 等）由模块 Manifest 消费，告警属正常，不影响启动 |
| 健康检查不通过 | `curl :8080/health/ready` 看 `checks`（pg/redis）；再看 `logs/<日期>/<module>_error.log` |
| 登录返回 503 `captcha not wired` | 登录验证码未装配（异常部署）——检查 realtime/identity 装配与 Redis 可用性 |
| 登录 400 `captcha invalid` | 验证码一次性：每次登录前重新 `GET /admin/v1/auth/captcha` |
| 接口 4013 | 401=token 过期/被吊销（refresh 或重登）；403=RBAC 权限码未授权 |
| 接口 429 | 限流或网关配额；网关 `Rate=0` 表示显式不限流，关闭配额用 `RateLimitEnabled=false` |
| WS 立即断开 | 关闭码 4001-4008 对照 [WS 协议 §7](../rules/ws-protocol.md)；4006=客户端发了协议外帧（如 hello） |
| 网关 18xxx | [状态码规范 · gateway 段](../rules/status-codes.md) + [网关签名契约](gateway-canonical.md) |

## 说明

- 文档随代码同仓演进；接口以 `contracts/openapi/*.yaml` 为准，冲突时以代码与契约为准。
- 选型与接入以本入口列出的文档为准；未列入的维护者内部资料不作为下游依据。

适用版本 v2.0.0
