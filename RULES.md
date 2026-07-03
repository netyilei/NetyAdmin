# RULES.md — 代码协作红线规范

> **作用**：AI 助手和开发者在每次修改代码前必须逐条检查本文件，违反任一条即为错误。
> **读取优先级**：本文件 → `docs/server-architecture.md` → `docs/admin-web-architecture.md` → `docs/status-codes.md`。
> **项目**：NetyAdmin（Go + Gin 后端 / Vue 3 + TypeScript 前端，BFF 多端隔离架构）。
> **最后更新**：2026-07-03

---

## 零、代码哲学（最优先）

### 0.1 不准写补丁代码

- 遇到需要「打补丁」才能绕过的逻辑缺陷，**必须重构整个相关逻辑**，而不是在现有代码上贴胶带。
- 补丁定义：为了绕过设计缺陷而添加的 `if` 特例、类型强转、全局变量后门、复制粘贴的相似函数。
- 如果发现某个模块的设计已无法承载当前需求 → 停下来，先改设计，再改代码。
- **基座程序只追求最优实现**：不考虑历史包袱、不为旧版本留兼容路径、不为旧数据开特例。需要兼容旧数据时，**走 migration 一次性迁移到位**，不在业务代码里写双路径分支。
- **发现既有补丁代码时直接重构**：禁止在补丁上叠补丁。在实现新功能/修 BUG 过程中遇到补丁形态的代码，优先做一次"补丁清理重构"，再继续新功能。
- 补丁代码的典型形态（命中即应重构）：
  1. 同一接口的多套实现并存且通过运行时开关选择（应统一为一套，或在编译期/装配期决定）
  2. 跨端/跨模块复制粘贴的逻辑（应提取到共享层）
  3. 公共函数为单一调用点加的特判/开关参数（应拆分为两个函数或在调用方处理）
  4. 为旧字段/旧格式留的桥接分支（应 migration 迁移数据 + 删除桥接）
  5. `// TODO` / `// 临时` / `// hack` / `// workaround` 注释标记的代码（应就地解决或纳入正式设计）

### 0.2 高聚合、低耦合（持续贯彻）

- 每个模块/包/文件只做一件事（SRP 单一职责）。
- 严格遵循分层调用链：`router -> handler -> service -> repository -> entity`。
- 模块间通过**接口**通信，不直接依赖具体实现。功能必须先封装为可复用单元，再对外暴露调用入口。
- **禁止在 handler 里写业务逻辑、在 service 里写 SQL、在 repository 里写业务判断**。
- **无侵入协议**：Service 层禁止出现 `*gin.Context`，只接收基础 Go 类型。

### 0.3 代码高度抽象，充分考虑扩展性

- 与外部系统（对象存储 / 短信 / 邮件 / 缓存 / 任务调度）的交互必须通过 **Adapter 接口** 隔离。
  - 存储驱动走 `internal/pkg/storage`，消息驱动走 `service/message` 的驱动化设计，缓存走 `pkg/cache` 双引擎抽象。
  - 新增实现 = 新增 Adapter，不修改调用方代码。
- 数据模型设计时考虑「后续加字段/加类型/加语言/加地区」——不在代码里写死枚举，用字典（`sys_dict_*`）或配置驱动。
- Admin 与 Client 端**物理隔离**：DTO、Handler、Router 各自独立存放，禁止跨端共享 DTO。

---

## 一、禁止事项（违反即错误）

### 1.1 硬编码

```go
// ❌ 禁止
client.Post("http://10.0.0.1:8001/internal/task/run")
if admin.RoleID == 1 { ... }

// ✅ 正确
client.Post(cfg.GetServiceURL("task") + "/internal/run")
if auth.HasPermission(admin.ID, "system:user:add") { ... }
```

### 1.2 魔法数字

```go
// ❌ 禁止
if article.Status == 0 { ... }
if retry > 3 { ... }

// ✅ 正确
if article.Status == ArticleStatusDraft { ... }
if retry > config.Task.MaxRetry { ... }
```

### 1.3 散落的常量

- 所有常量集中在各自模块的 `constants.go` 或共享的 `pkg/constants/` 中。
- 前端常量集中在 `src/constants/`，枚举集中在 `src/enum/`。
- 禁止在业务代码里直接写字符串比较、数字比较。

### 1.4 静默吞错误

```go
// ❌ 禁止
result, _ := repo.Find(ctx, id)
_ = cache.Set(ctx, key, val)

// ✅ 正确
result, err := repo.Find(ctx, id)
if err != nil {
    return nil, fmt.Errorf("find admin %d: %w", id, err)
}
```

### 1.5 handler 写业务逻辑

- Handler 只做三件事：参数绑定、调用 Service、组装响应。
- 所有业务判断、数据转换、外部调用放在 Service 层。
- SQL/DB 操作放在 Repository 层。

### 1.6 跨模块直接引用内部实现

- `internal/` 下的包禁止被其他模块直接 import 其 `*Impl` 或未导出类型。
- 只通过接口 + `wire.go` 注入。
- Admin 端的 Handler / DTO / Router 禁止被 Client 端引用，反之亦然。

### 1.7 Service 层出现 `*gin.Context`

- Service 层方法签名**禁止**出现 `*gin.Context`、`http.ResponseWriter` 等 HTTP 协议类型。
- 只接收 `context.Context` 与基础 Go 类型（DTO/ID/分页参数），保证 Service 可独立测试与复用。

---

## 二、必须遵守

### 2.1 文件规模

- 单文件不超过 **600 行**（含注释）。超过即拆分。
- 单函数不超过 **80 行**。超过即提取子函数。

### 2.2 注释规范

- **Go**：所有导出符号必须有文档注释（`// FuncName does X.`）。
- **业务逻辑**：复杂判断、状态转换、并发控制、权限校验必须有**中文注释**说明意图。
- **Swagger 注释**：所有 Handler 方法**必须**包含完整的 Swagger 注释，缺一不可：

  ```go
  // GetStorageConfig 获取存储配置详情
  // @Summary      获取存储配置详情
  // @Description  根据ID获取单个存储配置详情
  // @Tags         存储管理
  // @Accept       json
  // @Produce      json
  // @Param        id path int true "存储配置ID"
  // @Success      200 {object} response.Response "存储配置详情"
  // @Security     ApiKeyAuth
  // @Router       /admin/v1/storage-configs/{id} [get]
  ```

  必填项：`@Summary`、`@Description`、`@Tags`、`@Param`（有参数时）、`@Success`、`@Router`。Admin 端需登录接口还需 `@Security ApiKeyAuth`。
- **Vue/TypeScript**：复杂组件需有 JSDoc 注释说明用途；导出函数使用 `/** ... */` 注释。

### 2.3 命名规范

| 语言/范围 | 规范 |
|---|---|
| Go | 驼峰，导出首字母大写。缩写全大写（`HTTPServer`、`JWTAuth`、`URLPath`）|
| Go 文件名 | snake_case（`user_handler.go`、`operation_log.go`）|
| SQL 表/列 | snake_case，表名复数（`admins`、`articles`、`upload_records`）|
| Vue/TS 文件 | `kebab-case`（`article-form.vue`、`system-manage.ts`）|
| TS 类/接口 | PascalCase（`ArticleList`、`StorageConfig`）|
| API 类型 | 挂在 `ApiV1` 命名空间下，字段 snake_case 与后端 JSON 对齐 |

### 2.4 配置管理

- 所有可变参数（端口、超时、并发数、开关、密钥）走 `config.toml` + `internal/config/`。
- `.env.example` 记录所有环境变量，与 `config.toml` 一一对应。
- 禁止在代码里 `os.Getenv` 绕过 config 包。
- 前端环境变量通过 `.env.*` 注入，以 `VITE_` 前缀声明，并在 `src/typings/vite-env.d.ts` 补充类型。

### 2.5 错误处理（结构化错误码）

- 统一响应结构：`{ code, msg, data, request_id }`，`code` 为 6 位数字字符串。
- 错误码定义在 `internal/pkg/errorx/errorx.go`，编码规则见 `docs/status-codes.md`（`XX-YY-ZZ`：`10`=Admin 端，`20`=Client 端）。
- **Handler 层安全铁律**：
  - Service 返回的 `err`，统一用 `response.Fail(c, err)` 透传，**禁止**把 `err.Error()` 直接塞进响应体（会泄露内部细节）。
  - 已知错误码用 `response.FailWithCode(c, errorx.CodeXxx)`；需要自定义消息时 `response.FailWithCode(c, errorx.CodeXxx, "说明")`。
  - 需要自定义 HTTP 状态码时用 `response.FailWithStatus`。
  - 参数校验失败统一返回 `response.FailWithCode(c, errorx.CodeInvalidParams)`。
- Service 层返回 `error`，业务错误用 `errorx.New(errorx.CodeXxx)` 构造；底层错误逐层 wrap（`fmt.Errorf("context: %w", err)`）。
- 用 `errorx.Is(err, errorx.CodeXxx)` 判定业务错误类型，禁止用字符串比较错误消息。

### 2.6 安全规则（NetyAdmin 专属）

- **获取当前管理员 ID**：必须用 `c.GetUint("adminID")`，并校验 `if operatorID == 0` 兜底未授权场景。
  - 禁止用 `c.Get("adminID")` + `.(uint)` 类型断言（断言失败会 panic）。
- **路径参数 ID 解析**：必须用 `strconv.ParseUint(idStr, 10, 64)`，并检查 `err`；禁止用 `strconv.Atoi` 后忽略错误。
- **分页默认值**：分页请求必须兜底默认值：

  ```go
  if req.Current <= 0 {
      req.Current = 1
  }
  if req.Size <= 0 {
      req.Size = 20 // 或取自配置
  }
  ```
- **分页响应**：统一用 `response.SuccessWithPage(c, current, size, total, list)`，禁止手拼 `gin.H{"list":..., "total":...}`。
- **敏感字段脱敏**：password、token、AppSecret 等字段在日志输出前必须脱敏（由操作日志中间件统一处理，新代码不得绕过）。
- **权限分级**：严格区分公开接口、JWT 接口、RBAC 接口三级，新增接口必须在路由注册时挂载正确中间件。

### 2.7 依赖注入

- Go：统一用 **Wire**（`internal/app/wire.go`）。禁止在业务代码里 `NewXxx()` 自行构造依赖。
- 新增模块 → 新增 `ProviderSet` → 注册到 `wire.go`，并更新 `NewRouter` 等装配函数签名。
- 前端依赖通过构造函数注入，便于替换实现与单测 Mock。

### 2.8 数据库（Migration 铁律）

- 所有 DDL 变更通过 migration 文件，存放于 `internal/pkg/migration/migrations/`。**禁止手动改库**。
- 编号规则：`{前缀}_{描述}.sql`，`table_*.sql` 建表、`data_*.sql` 初始化数据。
- 已执行的 migration **绝对不可修改**。有变更 = 新增 migration。
- 软删除统一使用 `soft_delete.DeletedAt`（`BIGINT DEFAULT 0`），禁止混用 `gorm.DeletedAt`（TIMESTAMP）。
- 事务在 Repository 层处理，使用 `r.db.WithContext(ctx).Transaction(...)`。

### 2.9 测试（testify + 表驱动）

- Go 端统一使用 **`github.com/stretchr/testify`**（已在 `go.mod`）：
  - 断言用 `assert` / `require`（require 用于失败即停止的关键前置）。
  - Mock 用 `testify/mock`，禁止手写 fake 类。
  - 优先使用**表驱动测试**（table-driven test）组织用例：

    ```go
    tests := []struct {
        name    string
        input   int
        want    string
        wantErr bool
    }{
        {name: "valid", input: 1, want: "ok"},
        {name: "not found", input: 999, wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := svc.Get(ctx, tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
    ```
- 每个 Service 模块必须有对应的 `_test.go`；关键路径（鉴权、RBAC 校验、删除、上传凭证签发）必须有测试覆盖。
- 提交前跑通全部测试：`go test ./...`。
- 前端代码修改后必须通过 `pnpm typecheck` 与 `pnpm lint`，保持 0 Error、0 Warning。

### 2.10 禁止造轮子 — 必须使用已引入的能力

- 实现任何功能前，先确认是否已有对应基础设施，**已引入的必须使用，禁止手写替代实现**：
  - **Go**：缓存用 `pkg/cache`（Redis/BigCache 双引擎）+ gocache Tags 失效；限流用 `pkg/ratelimit`（ulule/limiter）；对象存储用 `pkg/storage`；任务调度用 `pkg/task`；密码加密用 `pkg/password`；JWT 用 `pkg/jwt`；配置用 `pelletier/go-toml`；UUID 用 `google/uuid`；ULID 用 `oklog/ulid`；验证码用 `pkg/captcha`（base64Captcha）；邮件用 `go-simple-mail`；DB 迁移用 `golang-migrate`；错误追踪用 `pkg/sentry`（getsentry/sentry-go + sentrygin）。
  - **前端**：UI 组件用 **Naive UI**，禁止手写下拉/弹窗/表格；HTTP 用 `service/request` 封装（@na/alova / @na/axios），禁止业务代码直接 `import axios`；状态用 **Pinia**；图标用 **Iconify**（unplugin-icons）；样式用 **UnoCSS**；国际化用 **vue-i18n**；错误追踪用 **@sentry/vue**（`src/plugins/sentry.ts`）。
- 若某功能确无已引入库覆盖，**先在文档中记录新增库并说明选型理由**，再写代码。

---

## 三、文档一致性（代码 ↔ 文档强制同步）

### 3.1 变更后必须同步的文档

| 变更类型 | 必须同步的文档 |
|---|---|
| 新增/修改 Client API | `docs/client-api-ws/*.md` |
| 新增/修改 Admin API | `docs/admin-api-ws/*.md` |
| 新增/修改 DB 表/字段 | `docs/server-architecture.md`（如涉及结构变更）+ migration SQL |
| 新增/修改错误码 | `docs/status-codes.md` + `internal/pkg/errorx/errorx.go` |
| 新增/修改配置项 | `config.toml` + `.env.example` |
| 新增/修改后端模块结构 | `docs/server-architecture.md` |
| 新增/修改前端模块结构 | `docs/admin-web-architecture.md` |
| 新增/修改 API 路由 | `docs/api-management.md` API 清单 |
| 新增/删除文档 | **必须更新 `README.md` 文档索引** |

### 3.2 API / WebSocket 定义权限

- `docs/client-api-ws/` 是 Client 端对外接口的唯一凭证，`docs/admin-api-ws/` 是 Admin 端对外接口的唯一凭证。
- API 格式以后端定义为主，后端编写。**前端只有读取权限，不可修改**。
- 前端若发现 API 不满足需求 → 提 issue → 后端更新对应 `*-api-ws/` → 前端再据此对接。
- 任何 API 变更必须先记入对应 `*-api-ws/` 文档，再改代码。

### 3.3 文档版本号

- 每次变更文档后，更新文档头部的「最后更新」日期。
- 主版本号变动规则：结构变更（新增/删除章节、重大重写）+1；内容补丁 +0.1。

---

## 四、Git 提交规范

```
<type>(<scope>): <subject>
```

- **type**：feat / fix / refactor / docs / test / chore / perf / ci
- **scope**：模块名（auth, rbac, content, storage, open-platform, ipac, message, captcha, task, dict, log, admin-web, ui, db 等）
- **subject**：中文描述，简洁（≤50 字），不加句号

示例：

```
feat(content): 新增文章定时发布任务调度
fix(rbac): 修复超级管理员角色被误删的校验遗漏
refactor(storage): 存储配置抽取驱动接口，支持多存储源切换
perf(cache): 上传记录缓存命中率优化
docs(api): 补充开放平台签名校验接口文档
ci(deploy): 新增 PostgreSQL 14 初始化脚本
```

---

## 五、分支与发布纪律

- `main` 分支 = 可部署状态。禁止直接向 main 提交。
- 功能开发在 `feature/<模块名>` 分支，完成后 PR → review → 合并。
- 紧急修复从 `main` 拉 `hotfix/<描述>`，修复后 PR → 合并回 `main` 和当前开发分支。
- 前端提交前由 `simple-git-hooks` 自动执行 `pnpm typecheck && pnpm lint`，不通过禁止提交。

---

## 六、AI 助手自检清单（每次修改前逐条确认）

1. ☐ 有没有写补丁代码？（有 → 停下来重构，不要贴胶带）
2. ☐ 有没有硬编码的字符串/数字/URL？
3. ☐ 有没有魔法数字？
4. ☐ 有没有常量散落在业务代码里？
5. ☐ 有没有静默吞掉 error？有没有把 `err.Error()` 直接塞进响应？
6. ☐ Handler 里有没有写业务逻辑？Service 里有没有出现 `*gin.Context`？
7. ☐ 获取 adminID 是不是用 `c.GetUint("adminID")`？路径 ID 是不是用 `strconv.ParseUint`？
8. ☐ 分页参数有没有兜底默认值？分页响应是不是用 `SuccessWithPage`？
9. ☐ 新文件超过 600 行了吗？单函数超过 80 行了吗？
10. ☐ Handler 有没有补全 Swagger 注释（@Summary/@Description/@Tags/@Param/@Success/@Router）？
11. ☐ 新配置走 config.toml 了吗？新依赖走 Wire 注入了吗？
12. ☐ DB 变更走 migration 了吗？
13. ☐ 改动后对应文档同步了吗？API 变更记入 `docs/*-api-ws/` 了吗？
14. ☐ Run 过测试了吗（`go test ./...` / `pnpm typecheck && pnpm lint`）？
15. ☐ 有没有造轮子？（要实现的功能是否已有引入的库/基础设施覆盖？有 → 用它，不手写）
16. ☐ 【联调/排错时】已经先查 Sentry Issues 和后端错误日志了吗？

---

## 七、错误追踪（联调排错第一入口）⭐ AI 必须优先使用

> **错误联调优先使用 Sentry**：联调时遇到任何前后端错误/异常，**第一步永远是查看 Sentry Issues 看板**，而非直接读代码猜测原因。通过 `request_id` 标签关联前后端链路，通过堆栈定位代码行，最后才是读代码修复。

### 7.1 技术选型

| 端 | 方案 | 说明 |
|---|---|---|
| Go 后端 | `getsentry/sentry-go v0.47.0` + `sentrygin` | 自动捕获 panic 和 Gin 上下文错误，带请求路径/方法/request_id/userID 上下文，经 `[sentry] dsn` 配置接入 |
| Admin-Web（Vue3） | `@sentry/vue` | 自动捕获前端异常 + 用户操作链路 + 浏览器环境信息，经 `VITE_SENTRY_DSN` 配置接入 |

### 7.2 强制规范

- **两端必须接入 Sentry**：Go 端通过 `sentry-go` 自动捕获 panic 和错误，Admin-Web 接入 `@sentry/vue`。DSN 为空时自动禁用，不影响正常启动。
- **关键路径必须打点**：所有 API 入口、开放平台签名校验、第三方/外部调用（对象存储、短信、邮件）、任务调度执行必须记录日志/事件。
- **错误必须包含上下文**：`error` 级别日志必须携带 `admin_id`/`user_id`、`app_id`、`request_id`（由 SentryTagSetter 中间件注入 Sentry Scope），便于跨端串联排查。
- **AI 联调排错优先使用**：AI 助手在排查任何 Bug 时，**必须优先查看 Sentry Issues 看板**（前端 `@sentry/vue` + 后端 `sentry-go`），再结合代码分析。不得跳过 Sentry 直接猜测原因。

### 7.3 Go 端 slog 使用规范

```go
import "log/slog"

// ✅ 正确：结构化日志 + 上下文
slog.Error("create storage config failed",
    slog.Uint("admin_id", operatorID),
    slog.String("request_id", requestID),
    slog.Any("error", err),
)

// ❌ 禁止：无结构化字段的裸日志
slog.Error("create failed")
```

### 7.4 Admin-Web Sentry 接入规范

- 通过 `VITE_SENTRY_DSN` 环境变量配置 DSN；未配置时不初始化，不得在代码里硬编码 DSN。
- 上线前必须配置 Release 版本号，便于区分线上版本异常。
- 用户登录后将 `admin_id`、`username` 设置到 Sentry User 上下文，登出时清理。

---

## 八、前端（Vue3）补充规范

- **组件优先**：UI 一律使用 Naive UI 现成组件（`NDataTable`、`NForm`、`NModal`、`NSelect` 等），禁止手写下拉/弹窗/表格/分页。
- **接口层严格分层**：
  - API 调用统一放 `src/service/api/v1/`，禁止在 `.vue` 组件里直接写 URL 或 `import axios`。
  - 类型定义统一放 `src/typings/api/v1/`，挂在 `ApiV1` 命名空间下，字段与后端 JSON 保持 snake_case 对齐。
- **国际化强制**：所有用户可见文本（菜单、按钮、提示、错误）必须走 i18n；后端返回的 `code` 由 `service/request/backend-error.ts` 映射为 `locales/langs/*/request.ts` 中的文案，禁止在组件里写死数字状态码或中文文案。
- **页面级组件隔离**：仅本页面使用的组件放 `views/<module>/components/`，禁止跨 `views/` 目录相互 import。
- **状态码收口**：业务代码中严禁写死数字状态码，统一用字典（`useDict`）或 `src/enum/` 中的枚举。
- **零死代码**：遵循 ESLint 规则，禁止未使用变量与导入，保持 `pnpm lint` 零警告。
- **错误追踪**：异常自动上报 `@sentry/vue`，关键操作失败需附带操作上下文。

---

## 九、BFF 架构铁律

- **Admin 与 Client 物理隔离**：接口层分属 `internal/interface/admin/` 与 `internal/interface/client/`，DTO、Handler、Router 各自独立，禁止共享 DTO。
- **权限模型不同**：Admin 走 `middleware.JWT() + middleware.RBAC()`，Client 走开放平台签名校验（`X-App-Key` + `X-Signature`）+ 基础 JWT。两套中间件不可混用。
- **路由前缀隔离**：Admin 端 `/admin/v1/*`，Client 端 `/client/v1/*`。
- **Service 可共享、接口层不可共享**：两端可复用同一 Service 实现，但 DTO 与 Handler 必须各写一套，保证入参结构与权限模型互不污染。

---

*本文件为 NetyAdmin 项目最高行为准则。所有参与者（人+AI）必须遵守。*
*违反本文件的代码，在 Code Review 阶段必须拒绝合并。*
