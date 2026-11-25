# NetyAdmin

**可二次开发的 Go 管理后台基座** —— 模块化单体架构，开箱即用的管理端 / 权限 / 平台服务，下游只写业务模块。

`v2.0.0` · Go 1.26+ · PostgreSQL 18 · Redis 8（single 可选） · Vue 3 · Apache-2.0

## 什么是 NetyAdmin

NetyAdmin 是一个**通用管理后台基座**，不是成品业务系统：它把每个后台项目都会重写一遍的东西（登录鉴权、RBAC 权限、配置中心、操作审计、文件直传、消息通知、实时推送、第三方开放平台网关）做成经过验证的内核与平台服务，并提供统一的模块契约（Manifest / Port / Event / Job / Hook / Migration）。

- **适合**：中后台 / 运营平台 / SaaS 管理面 / 需要清晰模块边界、可拆分部署与水平扩展的 Go 服务。
- **下游只写**：`internal/modules/<你的模块>` + 本地契约登记（`contracts/*.local.yaml`）+ 三处装配登记（assembly / migrate / 路由）；不改内核与平台服务。
- **非目标**：多租户（下游自行扩展）、C 端用户体系（`users` 为可替换示例）、插件市场（架构预留，当前不交付）。

## 设计初衷与理念

NetyAdmin 的出发点：**后台项目不该每次从登录鉴权开始重写**。把每个管理后台都会重复的部分（认证/RBAC/配置/审计/存储/消息/实时/开放平台）做成经过验证的内核与平台服务，业务从第一天就写自己的领域。

- **内核零业务**：`kernel` 只提供配置/数据/状态/总线/调度/Hook/安全/传输/观测/日志，不含业务概念；平台服务按需启用，业务模块可拆卸。
- **模块契约化**：模块用 Manifest 自描述（端口/事件/任务/迁移/配置/伸缩），模块间只走 `api/` 端口或事件，表归模块私有——换人、拆模块、拆部署不牵一发动全身。
- **契约先行**：状态码、命名空间、能力、迁移号段、WS 协议登记在 `contracts/`，代码用生成常量，CI 做对偶校验——接口与约定不靠口口相传。
- **多机默认**：模块默认无状态，会话/锁/计数/上传外置；`scaling.mode` 声明伸缩语义，滚动发布 N-1 兼容。
- **热更边界清晰**：配置版本化热更 + 广播 + 5 分钟对账；代码走滚动/蓝绿/金丝雀；DB 走 expand-contract，不做热补丁。
- **反过度设计**：新抽象/配置/依赖先答五问（谁在用/何时用/不用会怎样/最简方案/何时删除），答不出不做。

## 适用项目类型

**适合**

- 中后台管理平台 / 运营平台 / SaaS 管理面：登录、RBAC、菜单、配置、审计、上传、通知开箱即用。
- 开放平台网关：第三方应用（AppKey/AppSecret）、scope、HMAC 验签、配额、IP 访问控制、调用统计。
- 实时消息应用：WebSocket 三面（admin/client/gateway）、一次性 ticket、频道 ACL、心跳/背压/限流全配置。
- 需要长期演进的业务基座：模块边界清晰、可拆卸、可拆分部署，适合多年迭代与团队交接。
- 多团队共享平台：契约与权限数据随迁移交付，团队按模块并行开发，互不 import 内部实现。

**不适合**

- 纯静态站点、无服务端的展示页。
- 需要原生多租户（V2 非目标，下游自行扩展）。
- 一次性脚本 / 无长期维护的小工具——模块契约与门禁是长期成本。
- 微服务 / Service Mesh / 多地域双活 / Event Sourcing / CQRS / GraphQL / 动态插件 / 内置搜索 / 数仓（V2.0 非目标）。

## 核心能力

| 面 | 能力 |
|----|------|
| 认证与权限 | 管理员登录（验证码）/ JWT 轮换与吊销 / 首启随机密码 / 角色·菜单·API·按钮四级 RBAC / 动态路由 / 策略热更 |
| 管理端 | Vue 3 + Vite + TS（Element Plus），登录、系统管理、开放平台、任务中心等页面齐备，同源部署由后端直接服务 |
| 开放平台网关 | 应用（AppKey + AES-256-GCM AppSecret）/ scope 组 / API 登记 / IP 组集 / 配额 / HMAC-SHA256 验签 / 进程内转发 / 调用统计 |
| 实时推送 | WebSocket 三面（admin / client / gateway）：一次性 ticket 握手、频道两级 ACL、心跳/背压/限流全配置化 |
| 通知与存储 | 站内信 + 邮件(SMTP)/短信(log 驱动) + 模板；S3 兼容 presigned 直传 + HMAC 防伪 |
| 平台服务 | 动态配置（历史/回滚/广播）、操作审计（180 天保留）、任务调度（cron/interval、集群选主去重） |
| 内核设施 | Cache（L1+L2，entity class 声明制）、Bus（Local/Broadcast/Cluster + 事务发件箱）、分布式锁/幂等/选主、六段生命周期 Hook、统一信封/分页/限流/安全头、Prometheus + 健康检查 |
| 部署形态 | all-in-one → 按 role（all/core/admin）过滤接入面 → 拆分部署 → 模块级伸缩（`scaling.mode` 声明） |

## 架构一览

```
edges         adminapi /admin/v1 · clientapi /client/v1 · gateway /gateway/v1 · ops(health/metrics)
                 │  只向下依赖
business      users · note（示例）· 你的业务模块
platform      identity · rbac · configcenter · audit · storage · message · realtime · gw
kernel        config · data · state · bus · scheduler · hooks · security · transport · observability · log
```

- 依赖向内：`kernel ← platform/business ← edges`，禁止反向。
- 模块数据私有：模块间只走 `api/` Port 或领域事件，禁止跨模块 JOIN/事务/外键。
- 契约单一事实源：状态码 / 命名空间 / 能力 / 迁移号段全部登记在 `contracts/`，代码用生成常量。

| 目录 | 内容 |
|------|------|
| `cmd/netyadmin/` | 入口（serve / migrate / reset-password）与组合根 |
| `internal/kernel/` | 零业务内核（配置/数据/状态/总线/调度/Hook/安全/传输/观测/日志） |
| `internal/modules/` | 平台服务与业务模块（各含 `module.yaml` + 分层目录 + 迁移） |
| `admin/` | Vue 3 管理端（含 admin WS 客户端参考实现） |
| `contracts/` | 契约登记 + `openapi/` 三面 HTTP spec |
| `docs/` | 下游指南 / 架构设计 / ADR / 开发规范（入口见下方文档导航） |
| `deploy/` | 部署配置样例与说明 |

## 快速开始（5 分钟）

依赖：**Go ≥ 1.26**、**PostgreSQL 18**（库名 `netyadmin_v2`）、**Redis 8**（single 拓扑可不装，multi 必装）。管理端源码开发另需 Node ≥ 20 + pnpm。

> 每一行命令都可在 **[启动与运行完整手册](docs/guide/getting-started.md)** 找到详解（全部参数、环境变量、配置文件逐段说明、二进制部署、systemd、排错手册）。以下是最短路径。

```bash
# 0. 在仓库根目录操作（config.yaml / keys / logs / admin/dist 都相对工作目录解析）

# 1. 建库（用你的 PG 账号；并把 config.yaml 的 database.user 改成同一账号）
psql -h 127.0.0.1 -U <你的PG用户> -c "CREATE DATABASE netyadmin_v2"

# 2. 配置：复制样例并按需修改；密钥只走环境变量
cp deploy/config.example.yaml config.yaml
export NETYADMIN_DATABASE_PASSWORD=<你的PG密码>
export NETYADMIN_GW_SECRET_KEY=$(openssl rand -base64 32)   # 必设：应用密钥加密钥，缺失=拒绝启动

# 3. 迁移（新库全量 up；重复执行安全；首启随机密码在这里输出一次）
go run ./cmd/netyadmin -mode=migrate -migrate-action=up

# 4. 启动（默认 HTTP 8080）
go run ./cmd/netyadmin

# 5. 登录：浏览器打开 http:/7.0.0.1:8080 ，账号 admin
#    页面需要管理端产物 admin/dist（先 cd admin && pnpm install && pnpm build:pro），
#    或改用开发模式 cd admin && pnpm dev → http://localhost:5173
#    首启随机密码由第 3 步迁移命令输出一次；首次登录需修改密码；忘记密码：
#    go run ./cmd/netyadmin -mode=reset-password -username=admin
```

健康检查：`curl http:/7.0.0.1:8080/health/ready` → `{"status":"ok","checks":{"pg":"ok","redis":"ok"}}`。

**构建可分发二进制**（生产部署用，单文件 + 前端产物 + 配置）：

```bash
go build -o netyadmin ./cmd/netyadmin              # 当前平台
# 交叉编译：GOOS=linux GOARCH=amd64 go build -o netyadmin ./cmd/netyadmin
cd admin && pnpm build:pro                          # → admin/dist（后端同源挂载）
```

部署目录结构与启动顺序（含 staging/prod 必须预置 `keys/jwt_active.pem`）见 [启动与运行完整手册 §5](docs/guide/getting-started.md#5-二进制部署方式)。

管理端前端（源码开发）：`cd admin && pnpm install && pnpm dev` → http://localhost:5173（Vite 代理本机 8080）；生产用 `pnpm build:pro` 生成 `admin/dist`，后端启动时自动同源挂载。

## 命令行参数

```bash
netyadmin [-mode serve|migrate|reset-password] [-migrate-action up|status]
          [-username <管理员名>] [-config config.yaml] [-addr :8080] [-version]
```

| 参数 | 默认 | 说明 |
|------|------|------|
| `-mode` | `serve` | `serve` 启动服务 / `migrate` 迁移 / `reset-password` 重置管理员密码 |
| `-migrate-action` | `up` | `up` 应用全部迁移 / `status` 查看状态 |
| `-username` | — | `-mode=reset-password` 必填 |
| `-config` | `config.yaml` | 配置文件路径（不存在=纯默认值+环境变量） |
| `-addr` | — | 覆盖 `server.http_port` 的监听地址 |
| `-version` | — | 打印版本后退出 |

常用环境变量（全部 `NETYADMIN_` 前缀，完整表见 [启动手册 §7](docs/guide/getting-started.md#7-环境变量全表)）：

| 变量 | 必设 | 说明 |
|------|:----:|------|
| `NETYADMIN_DATABASE_PASSWORD` | 是 | PG 密码（其余 DB 参数在 `config.yaml`，有默认值） |
| `NETYADMIN_GW_SECRET_KEY` | 是 | base64 编码的 32 字节随机串；缺失/非法=启动失败 |
| `NETYADMIN_ENV` | 否 | `dev`（默认）/ `staging` / `prod`；prod 缺 JWT 密钥文件=拒绝启动 |
| `NETYADMIN_DEPLOYMENT_TOPOLOGY` | 否 | `single`（默认）/ `multi`；multi 必须 Redis，缺=启动失败 |
| `NETYADMIN_REDIS_ADDR` | multi 必设 | 如 `127.0.0.1:6379`；single 可空（降级进程内） |
| `NETYADMIN_SERVER_HTTP_PORT` | 否 | 默认 8080 |
| `NETYADMIN_MESSAGE_SMTP_PASSWORD` | 用邮件时 | 仅 env 注入 |
| `NETYADMIN_STORAGE_HMAC_SALT` | 用直传时 | 仅 env 注入；缺失=直传完成接口全拒 |
| `NETYADMIN_CACHE_L2_KEY` | 使用 PII 缓存时 | base64 32 字节；缺失=PII 实体注册失败（fail-closed） |

## 文档导航

**我要用起来（下游接入与二次开发）**

- [下游文档入口](docs/guide/README.md) —— 按任务导航：建模块 / 接网关 / 写 WS 客户端 / 部署 / 排障
- [启动与运行完整手册](docs/guide/getting-started.md) —— 全部参数 / 环境变量 / 配置文件 / 二进制部署 / 排错；从零到跑起来照抄即可
- [客户端接入指南](docs/guide/client-integration.md) —— 写 App / H5 / 第三方客户端时对接 HTTP API 与 WebSocket：三面选择、认证、ticket 握手、心跳重连、可运行示例
- [新手教程：从零做一个业务模块](docs/guide/tutorial-first-module.md) —— Step 0–9 完成 product 模块（脚手架 → 迁移 → 分层 → HTTP → 权限 → 装配 → 跑通）
- [下游开发指南](docs/guide/downstream-development.md) —— 建模块、用基座能力、HTTP/网关/WS 全流程操作手册
- [AI 辅助开发（下游开发提示词包）](docs/guide/downstream-development-rules.md) —— 把该文件交给你的 AI 编码助手（Claude Code / Cursor 等）并补上你的项目参数，即可按基座契约开始下游开发
- [网关 HMAC 签名契约](docs/guide/gateway-canonical.md) —— 任意语言实现第三方 SDK 的签名规范

**我要看接口契约**

- [HTTP API 契约](contracts/openapi/README.md) —— admin / client / gateway 三面 OpenAPI spec 与对接方式
- [状态码](docs/rules/status-codes.md) · [命名空间](docs/rules/namespaces.md) · [WS 协议](docs/rules/ws-protocol.md)
- 机器可读登记：[状态码](contracts/status-codes.yaml) · [命名空间](contracts/namespaces.yaml) · [迁移号段](contracts/migrations.yaml) · [能力目录](contracts/capabilities.yaml)

**我要理解设计与规范**

- 设计说明：`docs/guide/downstream-development.md`（分层/契约/状态平面）+ `docs/guide/downstream-development-rules.md`（基座边界与取舍）
- [开发规范（MUST/MUST NOT）](docs/rules/README.md) —— 状态码/缓存/迁移/日志/API 风格/安全/测试等

## 版本与许可

- 当前版本 **v2.0.0**（首个正式版）；对外能力变更见 [CHANGELOG.md](CHANGELOG.md)。
- 兼容承诺：Go 支持 N-2；模块 `api_version` minor 只加不删；HTTP 破坏式变更升路径版本（`/admin/v1`）；DB 迁移走 expand-contract。
- 许可证：[Apache 2.0](LICENSE)。

适用版本 v2.0.0
