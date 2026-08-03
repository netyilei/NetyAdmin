# Sentry 错误追踪模块详解

本文档详细介绍 NetyAdmin 的 Sentry 错误追踪系统，涵盖前后端集成、配置方式、调试工作流。

---

## 一、架构概览

本项目采用双端独立 Sentry 集成，覆盖前后端：

| 端 | 技术 | 集成位置 | 启用方式 |
|----|------|----------|----------|
| 后端 Go (server/) | `getsentry/sentry-go` + `sentrygin` | `internal/pkg/sentry/sentry.go` 初始化 + `middleware/recovery.go` 捕获 panic/错误 + `wire.go` 中间件链 | 需配置 `[sentry] dsn`（空则禁用） |
| 前端 Vue3 (admin-web/) | `@sentry/vue` | `src/plugins/sentry.ts` + `main.ts` 集成 + auth store 自动设置用户上下文 | 需配置 `VITE_SENTRY_DSN`（空则禁用） |

---

## 二、后端 Sentry 集成

### 2.1 核心文件

| 文件 | 作用 |
|------|------|
| `internal/pkg/sentry/sentry.go` | `Init()` 初始化 SDK、`Flush()` 刷新缓冲区、`CaptureException()` 手动捕获 |
| `internal/config/config.go` | `SentryConfig` 结构体 |
| `internal/app/wire.go` | Bootstrap 中初始化 Sentry + 注册 `sentrygin` 中间件 |
| `internal/app/app.go` | 退出时 `Flush(2s)` 确保事件提交 |
| `internal/middleware/recovery.go` | `SentryTagSetter` 注入 metadata；`ErrorLogger` 上报 Gin 错误 |

### 2.2 初始化条件

仅当 `config.toml` 的 `[sentry] dsn` 非空时 Sentry 才激活；为空则静默跳过，不影响正常启动。

### 2.3 配置项

```yaml
sentry:
  dsn: ""                        # 为空则禁用
  environment: development       # development / production
  release: "server@1.0.0"       # 版本号
  sample_rate: 1.0               # 错误事件采样率 (0.0-1.0)，不配置=默认1.0，显式0=关闭
  traces_sample_rate: 0.2        # 性能追踪采样率 (0.0-1.0)，0=关闭性能追踪
  # ignore_transactions 过滤高频低价值性能事务（regex），默认内置 /health /favicon /assets/
  # 用户配置会追加到默认清单之上
  # ignore_transactions:
  #   - "/api/v1/ping"
  #   - "/metrics"
```

### 2.4 中间件链顺序（关键）

```
RequestID → CORS → SecurityHeaders → Recovery → sentrygin(Repanic=true) → SentryTagSetter → ErrorLogger → ...
```

- `sentrygin` 在 `Recovery` 之后：panic 发生时先上报 Sentry，然后 Repanic 重新 panic，由外层 Recovery 兜底
- `SentryTagSetter` 在 `sentrygin` 之后：注入 `request_id`、`path`、`method`、`userID`
- `ErrorLogger` 同步上报 Gin 上下文错误到 Sentry

### 2.5 手动捕获

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

## 三、前端 Sentry 集成

### 3.1 核心文件

| 文件 | 作用 |
|------|------|
| `src/plugins/sentry.ts` | 核心插件：`setupSentry`、`captureError`、`setUserContext`、`clearUserContext` |
| `src/main.ts` | 在 `createApp(App)` 后立即调用 `setupSentry(app)` |
| `src/store/modules/auth/index.ts` | 登录成功后自动设置用户上下文，登出时清除 |

### 3.2 初始化条件

仅当 `.env` 中配置了 `VITE_SENTRY_DSN` 时 Sentry 才激活；未配置则静默跳过。

### 3.3 集成的功能

```typescript
Sentry.init({
  integrations: [
    browserTracingIntegration(),     // 性能追踪
    replayIntegration(),             // 会话回放
  ],
  tracesSampleRate:         生产 0.2 / 开发 1.0,
  replaysSessionSampleRate: 0.1,
  replaysOnErrorSampleRate: 1.0,
  ignoreErrors: [Network Error, timeout, ResizeObserver loop],
})
```

### 3.4 手动捕获

```typescript
import { captureError, setUserContext, clearUserContext } from '@/plugins/sentry'

// 手动上报错误
captureError(new Error('xxx'), { businessId: '123' })

// 设置/清除用户上下文（auth store 已自动处理，一般无需手动调用）
setUserContext({ id: '1', username: 'admin', role: 'admin' })
clearUserContext()
```

### 3.5 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `VITE_SENTRY_DSN` | Sentry DSN，为空则禁用 | `https://xxx@sentry.io/1` |
| `VITE_APP_VERSION` | 应用版本（作为 Sentry release） | `admin-web@1.0.0` |

---

## 四、调试工作流（联调时必须遵守）

```
遇到错误/异常
    │
    ▼
┌──────────────────────────────────────────────┐
│ 1. 检查 Sentry Issues                         │ ← AI 必须从这里开始
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

---

## 五、相关文档

- [Server 架构设计](./server-architecture.md)
- [Admin-Web 架构设计](./admin-web-architecture.md)
- [状态码规范](./status-codes.md)