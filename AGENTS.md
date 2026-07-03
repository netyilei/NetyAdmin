# AGENTS.md - NetyAdmin AI 协作指南

> 本文档供 AI 编程助手（Claude、GPT 等）参考，帮助快速理解 NetyAdmin 项目并高效协作。

---

## 1. 项目概述

NetyAdmin 是一个企业级后台管理系统基座，采用 **Go + Gin** 后端与 **Vue 3 + TypeScript** 前端，基于 **BFF (Backend For Frontend) 多端隔离架构** 构建。

- **后端**：按终端（Admin / Client）物理隔离接口层，严格分层 `router → handler → service → repository → entity`
- **前端**：严格页面层架构，API 按版本隔离（`v1/`），状态码与 i18n 解耦
- **核心能力**：RBAC 权限、JWT 认证、动态路由、透明缓存（Redis + BigCache）、PubSubBus 消息总线、LogBus 日志缓冲、任务调度、数据库自动迁移

---

## 2. 技术栈

### 后端（server/）

| 技术 | 版本 | 用途 |
|------|------|------|
| Go | 1.25.0 | 语言 |
| Gin | 1.12.0 | HTTP 框架 |
| GORM | 1.31.1 | ORM（PostgreSQL） |
| go-redis | v9.18.0 | Redis 客户端 |
| BigCache | v3.1.0 | 本地一级缓存（L1） |
| golang-migrate | v4.18.2 | 数据库迁移（SQL 文件） |
| golang-jwt | v5.3.1 | JWT 认证 |
| swaggo/swag | v1.16.6 | Swagger 文档生成 |
| stretchr/testify | v1.11.1 | 测试框架 |
| ulule/limiter | v3.11.2 | 限流 |
| go-simple-mail | v2.16.0 | 邮件发送 |
| minio-go | v7.2.1 | 对象存储 SDK |
| soft_delete | v1.2.1 | GORM 软删除插件（BIGINT 类型） |

### 前端（admin-web/）

| 技术 | 版本 | 用途 |
|------|------|------|
| Vue | 3.5.17 | 框架 |
| TypeScript | 5.8.3 | 类型系统 |
| Vite | 7.0.4 | 构建工具 |
| Naive UI | 2.42.0 | UI 组件库 |
| UnoCSS | 66.3.3 | 原子化 CSS |
| Pinia | 3.0.3 | 状态管理 |
| Vue Router | 4.5.1 | 路由 |
| vue-i18n | 11.1.9 | 国际化 |
| @sentry/vue | ^10.63.0 | 前端错误追踪（按需启用） |
| Node.js | >= 20.19.0 | 运行环境 |
| pnpm | >= 10.5.0 | 包管理器 |

---

## 3. 项目结构

```
NetyAdmin/
├── server/                          # 后端服务（Go + Gin）
│   ├── cmd/server/main.go          # 进程入口
│   ├── config.toml                  # 运行配置
│   ├── docs/                        # Swagger 生成产物（docs.go, swagger.json）
│   └── internal/
│       ├── app/                     # 应用启动与依赖装配
│       │   ├── app.go              # 生命周期管理
│       │   ├── init.go             # DB 初始化
│       │   └── wire.go             # 依赖注入（Bootstrap 函数）
│       ├── config/                  # 配置结构体
│       ├── domain/                  # 领域模型
│       │   ├── entity/             # GORM 实体
│       │   └── vo/                 # View Object
│       ├── interface/              # 接入层（BFF 端隔离）
│       │   ├── admin/             # Admin 端（dto + http/handler + http/router）
│       │   └── client/            # Client 端（dto + http/handler + http/router）
│       ├── middleware/             # Gin 中间件
│       ├── pkg/                    # 基础设施包（errorx, response, cache, jwt 等）
│       ├── repository/              # 数据访问层
│       ├── service/                 # 业务服务层
│       └── job/                     # 内置定时任务
│
├── admin-web/                       # 管理后台前端（Vue 3）
│   ├── src/
│   │   ├── service/api/v1/         # API 封装（按版本隔离）
│   │   ├── service/request/        # Axios 请求封装
│   │   ├── typings/api/v1/         # API 类型定义
│   │   ├── views/                  # 页面（严格页面层）
│   │   ├── store/modules/          # Pinia 状态模块
│   │   ├── components/             # 通用组件
│   │   ├── hooks/                  # 组合式函数
│   │   ├── locales/langs/          # 国际化资源
│   │   ├── router/                 # 路由配置
│   │   └── layouts/                # 布局组件
│   ├── packages/                   # pnpm workspace 内部包
│   └── .env / .env.test / .env.prod
│
└── docs/                            # 项目文档
```

---

## 4. 后端开发指南

新增一个业务模块需遵循以下 7 步（以"评论管理"为例）：

| 步骤 | 层级 | 文件路径 | 说明 |
|------|------|----------|------|
| 1 | Entity | `internal/domain/entity/<module>/xxx.go` | GORM 实体，使用 `soft_delete.DeletedAt`（BIGINT） |
| 2 | Repository | `internal/repository/<module>/xxx.go` | 定义接口 + 实现，注入 `*gorm.DB` |
| 3 | DTO | `internal/interface/admin/dto/<module>/xxx.go` | 请求/响应结构体，带 `binding` 校验标签 |
| 4 | Service | `internal/service/<module>/xxx.go` | 业务逻辑，定义接口 + 实现，**禁止出现 `*gin.Context`** |
| 5 | Handler | `internal/interface/admin/http/handler/v1/<module>/xxx_handler.go` | 参数绑定 → 调 Service → 统一响应，**必须写 Swagger 注解** |
| 6 | Router | `internal/interface/admin/http/router/v1/<module>.go` | 注册路由到 `publicGroup` / `authGroup` / `permissionGroup` |
| 7 | Wire | `internal/app/wire.go` | 在 `initRepositories`、`initServices`、`initHandlers` 中添加注入，并传入 `NewRouter` |

### 关键约束

- **Service 层无侵入**：只接收基础 Go 类型（`context.Context`, `uint`, `string` 等），禁止传入 `*gin.Context`
- **DTO 端隔离**：Admin 和 Client 的 DTO 独立存放，禁止全局共享
- **接口先行**：Repository 和 Service 均定义 interface，便于 mock 测试

---

## 5. 前端开发指南

新增一个前端页面需遵循以下步骤：

| 步骤 | 位置 | 说明 |
|------|------|------|
| 1 | `src/typings/api/v1/<module>.d.ts` | 定义 API 类型（Request / Response） |
| 2 | `src/service/api/v1/<module>.ts` | 封装 API 函数，统一用 `request<T>()` 调用 |
| 3 | `src/views/<module>/index.vue` | 页面入口；页面专属组件放 `views/<module>/components/` |
| 4 | `src/locales/langs/zh-cn/` + `en-us/` | 添加国际化文案（route.ts、page/ 等） |

### 关键约束

- **禁止跨 views 引用**：`views/manage/` 不可 import `views/content/` 下的组件
- **禁止硬编码 URL**：`.vue` 文件中不允许直接调用 axios 或写死接口地址
- **禁止硬编码状态码**：后端返回 code，前端通过 `locales/langs/.../request.ts` 映射为 i18n 文本
- **命名规范**：Vue 文件和普通 TS 文件用 `kebab-case`，类/接口用 `PascalCase`
- 路由默认由后端动态返回，无需前端硬编码；本地测试可在 `src/router/routes/builtin.ts` 临时添加

---

## 6. 代码规范要点

### 分层架构

严格遵循调用链，禁止跨层调用或反向依赖：

```
Router → Handler → Service → Repository → Entity
```

### 错误处理（errorx）

| 场景 | 做法 |
|------|------|
| Service 层返回业务错误 | `return nil, errorx.New(errorx.CodeUserNotFound)` |
| Handler 处理 Service 错误 | `response.Fail(c, err)` — 自动识别 `BizError` 并返回对应 code |
| Handler 参数校验失败 | `response.FailWithCode(c, errorx.CodeInvalidParams)` |
| Handler 成功返回 | `response.Success(c, data)` |
| 分页返回 | `response.SuccessWithPage(c, current, size, total, list)` |

**错误码定义**：位于 `internal/pkg/errorx/errorx.go`，编码规则 `1XXYYY`（XX=模块号，YYY=具体错误）。新增错误码需同步更新 `codeMessages` map。

### 统一响应格式

所有接口返回 HTTP 200，通过 `code` 字段区分业务结果：

```json
{
  "code": "100000",
  "msg": "",
  "data": {},
  "request_id": "xxx"
}
```

分页数据使用 `PageData` 结构（`records` / `current` / `size` / `total`）。

### 依赖注入（wire.go）

> 注意：本项目 **未使用 Google Wire 代码生成**，而是在 `internal/app/wire.go` 的 `Bootstrap()` 函数中手动装配依赖。三个初始化函数：
> - `initRepositories(db)` — 装配所有 Repository
> - `initServices(...)` — 装配所有 Service
> - `initHandlers(...)` — 装配所有 Handler
>
> 新增模块需在这三个函数中添加对应实例化代码，并在 `NewRouter()` 调用处传入新的 Handler。

### 数据库迁移规范

- 迁移工具：golang-migrate，SQL 文件通过 `go:embed` 编译进二进制
- 文件位置：`internal/pkg/migration/migrations/`
- 命名格式：`NNNN_name.up.sql` / `NNNN_name.down.sql`（序号递增，如 `0058_xxx`）
- 新建迁移命令：

```bash
migrate create -ext sql -dir internal/pkg/migration/migrations -seq <name>
```

- 表结构用 `table_` 前缀，种子数据用 `seed_` 前缀，外键约束用 `fk_` 前缀
- 启动时自动执行（受 `config.toml` 的 `[migration] enabled` 控制）

### Swagger 注解

每个 Handler 方法**必须**编写 Swagger 注解，格式见 [第 8 节](#8-swaggerapi-文档)。

---

## 7. 测试规范

| 规范 | 说明 |
|------|------|
| 测试框架 | `github.com/stretchr/testify`（主要用 `assert`） |
| 文件命名 | `xxx_test.go`，与被测文件同目录同包（外部测试包 `xxx_test`） |
| 测试风格 | **表驱动测试**（table-driven tests），用 `[]struct{name; input; want}` 组织用例 |
| 子测试 | 使用 `t.Run(tt.name, func(t *testing.T) {...})` |
| 断言 | `assert.Equal(t, expected, actual)`、`assert.True/False`、`assert.NoError` |

参考实现：`internal/pkg/errorx/errorx_test.go`

```go
func TestCode_Message(t *testing.T) {
    tests := []struct {
        name string
        code errorx.Code
        want string
    }{
        {"success", errorx.CodeSuccess, "操作成功"},
        {"not found", errorx.CodeNotFound, "资源不存在"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.want, tt.code.Message())
        })
    }
}
```

运行测试：

```bash
cd server
go test ./...                          # 全部测试
go test ./internal/pkg/errorx/... -v   # 指定包，详细输出
```

---

## 8. Swagger/API 文档

### 注解编写

在每个 Handler 方法上方编写标准 Swagger 注解：

```go
// @Summary      获取文章列表
// @Description  分页获取文章列表，支持多条件筛选
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Success      200 {object} response.Response "文章列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/articles [get]
func (h *ContentArticleHandler) List(c *gin.Context) { ... }
```

### 全局信息

`cmd/server/main.go` 中定义全局 Swagger 元信息（`@title`、`@version`、`@BasePath` 等）。

### 生成与访问

```bash
cd server
swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
```

- 生成产物：`server/docs/`（`docs.go`、`swagger.json`）
- 访问地址：`http://localhost:9090/swagger/index.html`（仅 `debug` 模式开放）
- 路由注册：`internal/interface/admin/http/router/router.go`

---

## 9. 错误追踪（Sentry）⭐ 联调优先

> **联调/排查问题时，AI 助手应优先检查 Sentry 错误记录**，而非自行猜测问题原因。
> 先看 Sentry Issues 看板上是否有对应错误 → 再结合堆栈和上下文日志分析 → 最后才是读代码。

### 9.1 架构概览

本项目采用双端独立 Sentry 集成，覆盖前后端：

| 端 | 技术 | 集成位置 | 启用方式 |
|----|------|----------|----------|
| 后端 Go (server/) | `getsentry/sentry-go v0.47.0` + `sentrygin` | `internal/pkg/sentry/sentry.go` 初始化 + `middleware/recovery.go` 捕获 panic/错误 + `wire.go` 中间件链 | 需配置 `[sentry] dsn`（空则禁用） |
| 前端 Vue3 (admin-web/) | `@sentry/vue` | `src/plugins/sentry.ts` + `main.ts` 集成 + auth store 自动设置用户上下文 | 需配置 `VITE_SENTRY_DSN` |

### 9.2 后端 Sentry 集成详情

**文件位置**：

| 文件 | 作用 |
|------|------|
| `internal/pkg/sentry/sentry.go` | 核心包：`Init()` 初始化 SDK、`Flush()` 刷新缓冲区、`CaptureException()` 手动捕获 |
| `internal/config/config.go` | `SentryConfig` 结构体（DSN / Environment / Release / SampleRate / TracesSampleRate） |
| `internal/app/wire.go` | Bootstrap 中初始化 Sentry + 注册 `sentrygin` 中间件 |
| `internal/app/app.go` | 退出时 `Flush(2s)` 确保事件提交 |
| `internal/middleware/recovery.go` | `SentryTagSetter` 注入 requestID/path/userID 到 Sentry Scope；`ErrorLogger` 同步上报 Gin 错误到 Sentry |

**中间件链顺序**（关键）：

```
RequestID → CORS → SecurityHeaders → Recovery → sentrygin(Repanic=true) → SentryTagSetter → ErrorLogger → ...
```

- `sentrygin` 在 `Recovery` 之后：panic 发生时 sentrygin 先捕获并上报 Sentry（带请求上下文），然后 Repanic 重新 panic，由外层 Recovery 兜底记录到 DB error_log 表并返回 500
- `SentryTagSetter` 在 `sentrygin` 之后：将 `request_id`、`path`、`method`、`userID` 注入 Sentry Scope，实现前后端链路关联
- `ErrorLogger` 同步上报 Gin 上下文错误到 Sentry（若 hub 存在用 hub，否则用全局 hub）

**初始化条件**：仅当 `config.toml` 的 `[sentry] dsn` 非空时 Sentry 才激活；为空则静默跳过，不影响正常启动。

**配置项**：

```toml
[sentry]
dsn = ""                        # 为空则禁用
environment = "development"    # development / production
release = "server@1.0.0"      # 版本号
# sample_rate：未配置（删除该行）=默认 1.0；显式 0.0=关闭错误上报（v1.0.1 修复）
sample_rate = 1.0              # 错误事件采样率 (0.0-1.0)
traces_sample_rate = 0.2       # 性能追踪采样率 (0.0-1.0)，0=关闭性能追踪
# ignore_transactions：过滤高频低价值性能事务（regex），默认内置 /health /favicon /assets/
# 用户配置会追加到默认清单之上（v1.0.2 新增）
# ignore_transactions = ["/api/v1/ping", "/metrics"]
```

**手动捕获错误**（Go 代码中）：

```go
import pkgSentry "NetyAdmin/internal/pkg/sentry"

// 手动上报错误
pkgSentry.CaptureException(err)

// 手动上报消息
pkgSentry.CaptureMessage("something went wrong")

// 设置全局标签
pkgSentry.SetTag("module", "content")
```

### 9.3 前端 Sentry 集成详情

**文件位置**：

| 文件 | 作用 |
|------|------|
| `src/plugins/sentry.ts` | 核心插件：初始化 Sentry，导出 `setupSentry`、`captureError`、`setUserContext`、`clearUserContext` |
| `src/main.ts` | 在 `createApp(App)` 后立即调用 `setupSentry(app)` |
| `src/store/modules/auth/index.ts` | 登录成功后自动设置用户上下文，登出时清除 |

**初始化条件**：仅当 `.env` 中配置了 `VITE_SENTRY_DSN` 时 Sentry 才激活；未配置则静默跳过，不影响正常使用。

**集成的功能**：

```
Sentry.init({
  integrations: [
    browserTracingIntegration(),     // 性能追踪
    replayIntegration(),             // 会话回放
  ],
  tracesSampleRate:         生产 0.2 / 开发 1.0,
  replaysSessionSampleRate: 0.1,
  replaysOnErrorSampleRate: 1.0,
  ignoreErrors: [Network Error, timeout, ResizeObserver loop],  // 忽略非操作类错误
})
```

### 9.4 调试工作流（联调时 AI 必须遵守）

```
遇到错误/异常
    │
    ▼
┌──────────────────────────────────────────────┐
│ 1. 检查 Sentry Issues                         │ ← AI 优先从这里开始
│    - 是否有对应错误？堆栈信息是什么？          │
│    - 前端错误：浏览器/版本/操作系统            │
│    - 后端错误：请求路径/方法/堆栈             │
│    - 用户上下文（ID / 角色）                  │
│    - 上下游 Span（Trace ID）+ request_id 关联  │
│    - 区分前端(@sentry/vue) vs 后端(sentry-go)  │
└──────────────────────────────────────────────┘
    │ 有匹配
    ▼
┌──────────────────────────────────────────────┐
│ 2. 结合后端日志定位                           │
│    - 看 server/ 的 log/slog 输出              │
│    - 查 DB error_log 表中的上下文              │
│    - 用 request_id 关联前后端链路              │
└──────────────────────────────────────────────┘
    │
    ▼
┌──────────────────────────────────────────────┐
│ 3. 修复后验证                                 │
│    - Sentry Issues 不再新增同类错误            │
│    - 手动复现确认                             │
└──────────────────────────────────────────────┘
```

### 9.5 环境变量与配置

**前端**（`.env` 文件）：

| 变量 | 说明 | 示例 |
|------|------|------|
| `VITE_SENTRY_DSN` | Sentry DSN，为空则禁用 | `https://xxx@sentry.io/1` |
| `VITE_APP_VERSION` | 应用版本（作为 Sentry release） | `admin-web@1.0.0` |

**后端**（`config.toml`）：

| 配置项 | 说明 | 示例 |
|--------|------|------|
| `[sentry] dsn` | Sentry DSN，为空则禁用 | `https://xxx@sentry.io/2` |
| `[sentry] environment` | 环境标识 | `development` |
| `[sentry] release` | 版本号 | `server@1.0.0` |
| `[sentry] sample_rate` | 错误采样率（未配置默认 1.0，显式 0 关闭） | `1.0` |
| `[sentry] traces_sample_rate` | 性能追踪采样率（0 关闭性能追踪） | `0.2` |
| `[sentry] ignore_transactions` | 过滤高频低价值性能事务（regex，默认含 /health 等） | （可选） |

### 9.6 手动捕获错误

**前端**（TypeScript）：

```typescript
import { captureError, setUserContext, clearUserContext } from '@/plugins/sentry';

// 手动上报错误
captureError(new Error('xxx'), { businessId: '123' });

// 设置/清除用户上下文（auth store 已自动处理，一般无需手动调用）
setUserContext({ id: '1', username: 'admin', role: 'admin' });
clearUserContext();
```

**后端**（Go）：

```go
import pkgSentry "NetyAdmin/internal/pkg/sentry"

// 手动上报错误（Sentry 未初始化时为空操作，安全调用）
pkgSentry.CaptureException(err)

// 手动上报消息
pkgSentry.CaptureMessage("定时任务执行异常")

// 设置全局标签
pkgSentry.SetTag("module", "content")
```

---

## 10. 常用命令

### 后端（server/）

```bash
# 运行
go run cmd/server/main.go

# 构建
go build -o bin/server cmd/server/main.go

# 测试
go test ./...
go test ./internal/pkg/errorx/... -v

# 生成 Swagger 文档
swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

# 新建数据库迁移
migrate create -ext sql -dir internal/pkg/migration/migrations -seq <name>
```

### 前端（admin-web/）

```bash
# 安装依赖
pnpm install

# 开发（默认 test 模式，连接 localhost:9090）
pnpm dev

# 生产模式开发
pnpm dev:prod

# 构建
pnpm build          # 生产环境
pnpm build:test     # 测试环境

# 代码检查
pnpm lint           # ESLint 修复
pnpm typecheck      # TypeScript 类型检查

# 预览构建结果
pnpm preview
```

### 默认账号

- 账号：`admin`
- 密码：`admin123`

---

## 11. 环境配置

### 后端 config.toml

位于 `server/config.toml`，主要配置段：

| 配置段 | 说明 |
|--------|------|
| `[server]` | 端口（9090）、模式（debug/release）、超时、多节点标识 |
| `[database]` | PostgreSQL 连接（host/port/user/password/dbname/sslmode/连接池） |
| `[redis]` | Redis 开关、前缀、连接、L1 缓存（BigCache）参数 |
| `[jwt]` | JWT 密钥与过期时间（小时） |
| `[migration]` | 迁移开关 |
| `[task]` | 任务调度开关、Worker 数、各任务 cron 配置 |
| `[security]` | AES 密钥（32 字节 = AES-256） |
| `[email]` | SMTP 配置（SSL/STARTTLS/AuthType） |
| `[bus]` | 事件总线驱动（memory/redis，不设置则根据 Redis 自动选择） |
| `[sentry]` | Sentry 错误追踪（DSN 为空则禁用，environment/release/采样率/事务过滤） |

### 前端 .env 文件

| 文件 | 说明 |
|------|------|
| `.env` | 基础配置（标题、图标前缀、路由模式、成功码等） |
| `.env.test` | 测试环境（`VITE_SERVICE_BASE_URL=http://localhost:9090`） |
| `.env.prod` | 生产环境（后端地址） |

关键环境变量：

| 变量 | 说明 |
|------|------|
| `VITE_SERVICE_BASE_URL` | 后端 API 基地址 |
| `VITE_SERVICE_SUCCESS_CODE` | 成功码（`100000`） |
| `VITE_AUTH_ROUTE_MODE` | 路由模式（static/dynamic） |
| `VITE_HTTP_PROXY` | 是否启用开发代理 |
| `VITE_SOURCE_MAP` | 是否生成 sourcemap |
| `VITE_SENTRY_DSN` | Sentry DSN（为空则禁用前端错误追踪） |

---

## 12. 注意事项

> **错误联调优先使用 Sentry**：联调时遇到任何前后端错误/异常，**第一步永远是查看 Sentry Issues 看板**（前端 `@sentry/vue` + 后端 `sentry-go`），而非直接读代码猜测原因。通过 `request_id` 标签关联前后端链路，通过堆栈定位代码行，最后才是读代码修复。详见 [§9 错误追踪（Sentry）](#9-错误追踪sentry-联调优先)。

### 安全规范

| 规则 | 正确做法 | 错误做法 |
|------|----------|----------|
| **错误联调优先用 Sentry** | 联调时先看 Sentry Issues 是否有对应错误，通过 `request_id` 关联前后端 | 不看 Sentry 直接猜测问题原因 |
| **不泄露内部错误** | Service 返回 `errorx.New(code)`，Handler 用 `response.Fail(c, err)` | `response.FailWithCode(c, errorx.CodeInternalError, err.Error())` |
| **获取用户 ID** | `c.GetUint("adminID")` | `c.Get("adminID").(uint)`（panic 风险） |
| **Service 错误处理** | `response.Fail(c, err)` 自动识别 BizError | 手动判断 err 类型或暴露原始错误 |
| **分页参数校验** | 始终校验并设置默认值（current>=1, size 合理范围） | 直接信任前端传入的分页参数 |
| **密码/Token** | 日志中间件自动脱敏 | 在日志或响应中输出敏感字段 |

### 架构红线

- **Service 层禁止出现 `*gin.Context`** — 只接收基础 Go 类型
- **DTO 禁止跨端共享** — Admin/Client 各自独立
- **Handler 只做协议转换** — 参数绑定 + 调 Service + 统一响应，不含业务逻辑
- **Repository 只做数据访问** — 不含业务规则，事务在此层管理
- **软删除统一用 `soft_delete.DeletedAt`**（BIGINT 类型），不用 `gorm.DeletedAt`（TIMESTAMP）

### 多节点部署

- `multi_node = true` 时，`[bus] driver` 必须设为 `"redis"`，否则缓存/IPAC/配置失效不会跨节点同步（启动时会打印 WARN）
- 分布式锁依赖 Redis，Redis 不可用时自动降级为进程内内存限流

### 前端代码质量

- 保持 **0 Error, 0 Warning** 状态
- 提交前自动执行 `pnpm typecheck && pnpm lint`（git pre-commit hook）
- 严格 ESLint 规则，禁止未使用变量和导入
