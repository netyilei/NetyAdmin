# Server 架构设计与目录结构

本文档详细介绍 NetyAdmin 服务端（Go）的架构设计理念、分层结构和二次开发指南。

---

## 一、架构设计理念

### 1.1 设计目标

NetyAdmin Server 采用 **BFF (Backend For Frontend) 多端隔离架构**，旨在解决以下问题：

- **业务逻辑差异**：不同终端（Admin、Client）的登录逻辑、入参 DTO 完全不同，混合存放会导致命名混乱
- **权限安全隔离**：Admin 需要 RBAC 中间件，Client 只需要基础 JWT，混合配置容易产生越权漏洞
- **团队并行开发**：各端开发者互不影响，各自演进

### 1.2 核心设计原则

| 原则 | 说明 |
|------|------|
| **端隔离** | 按终端类型（admin/client）物理隔离接口层，DTO 独立存放 |
| **分层清晰** | 严格遵循 `router → handler → service → repository → entity` 单向调用链 |
| **无侵入协议** | Service 层禁止出现 `*gin.Context`，只接收基础 Go 类型 |
| **DTO 专属** | 每个端的 DTO 独立存放，禁止跨端共享；Service 接口接收 DTO，不接收 entity |
| **依赖注入** | 使用 Wire 进行依赖装配，便于测试和替换实现 |
| **TM 事务管理** | 所有多步原子操作统一使用 `TransactionManager`，Repository 不自管事务 |
| **缓存失效在后** | 缓存失效必须在 `tm.Commit()` 之后执行，避免回滚后缓存已清 |
| **fail-closed** | 敏感操作（删除/禁用/改密）使用 TM 单事务原子完成，失败不回补 |

---

## 二、目录结构详解

```
server/
├── cmd/server/                    # 进程入口
│   └── main.go                    # 加载配置 -> 初始化DB -> 启动服务
│
├── config.toml                    # 运行配置（TOML格式）
├── go.mod / go.sum               # Go模块依赖
│
├── internal/pkg/migration/migrations/   # SQL迁移脚本
│   ├── table_*.sql               # 表结构定义
│   └── data_*.sql                # 基础数据
│
└── internal/                      # 私有业务代码（不对外暴露）
    ├── app/                       # 应用启动与依赖装配
    │   ├── app.go                # 应用生命周期管理
    │   ├── init.go               # 初始化逻辑（DB、Redis等）
    │   └── wire.go               # Wire依赖注入配置
    │
    ├── config/                    # 配置结构与加载
    │   └── config.go             # TOML配置结构体定义
    │
    ├── domain/                    # 领域模型层
    │   ├── entity/               # 持久化实体（GORM Model）
    │   │   ├── base.go           # 基础实体（ID、时间戳等）
    │   │   ├── content/          # 内容管理实体
    │   │   ├── log/              # 日志实体
    │   │   ├── open_platform/    # 开放平台实体（App、ScopeGroup、OpenLog）
    │   │   ├── storage/          # 存储实体（Config、Record含AppID）
    │   │   └── system/           # 系统管理实体
    │   │
    │   └── vo/                   # 面向前端的View Object
    │       ├── content/
    │       ├── log/
    │       └── system/
    │
    ├── interface/                 # 【接入层】按端隔离
    │   ├── admin/                # 面向Admin-Web的接口
    │   │   ├── dto/              # Admin专用DTO
    │   │   │   ├── content/      # 内容管理DTO
    │   │   │   ├── log/          # 日志DTO
    │   │   │   ├── open_platform/ # 开放平台DTO（含StorageID）
    │   │   │   ├── storage/      # 存储DTO
    │   │   │   └── system/       # 系统管理DTO
    │   │   │
    │   │   └── http/             # HTTP协议接入
    │   │       ├── handler/v1/   # Handler实现
    │   │       │   ├── admin/    # 管理员相关
    │   │       │   ├── auth/     # 认证相关
    │   │       │   ├── content/  # 内容管理
    │   │       │   ├── log/      # 日志管理
    │   │       │   ├── open_platform/ # 开放平台管理
    │   │       │   ├── storage/  # 存储管理
    │   │       │   └── system/   # 系统管理
    │   │       │
    │   │       └── router/v1/    # 路由注册
    │   │           ├── admin.go
    │   │           ├── auth.go
    │   │           ├── content.go
    │   │           ├── log.go
    │   │           ├── open_platform.go
    │   │           ├── router.go # 路由聚合入口
    │   │           ├── storage.go
    │   │           └── system.go
    │   │
    │   └── client/               # 面向Client端的接口
    │       ├── dto/v1/           # Client专用DTO
    │       │   └── storage.go    # 存储上传DTO
    │       │
    │       └── http/             # HTTP协议接入
    │           ├── handler/v1/   # Handler实现
    │           │   ├── auth_handler.go
    │           │   ├── content_handler.go
    │           │   ├── echo_handler.go
    │           │   ├── message_handler.go
    │           │   ├── storage_handler.go
    │           │   └── user_handler.go
    │           │
    │           └── router/v1/    # 路由注册
    │               ├── auth_router.go
    │               ├── content_router.go
    │               ├── echo_router.go
    │               ├── message_router.go
    │               ├── router.go
    │               ├── storage_router.go
    │               └── user_router.go
    │
    ├── job/                       # 内置任务
    │   ├── article_publish.go    # 文章定时发布
    │   ├── init.go               # 任务注册入口
    │   └── system_log_cleanup.go # 日志清理
    │
    ├── middleware/                # Gin中间件
    │   ├── auth.go               # JWT认证（薄壳，注入 ClaimsAccessor 到 pkg/auth.RequireAuth）
    │   ├── open_platform_auth.go # 开放平台签名校验
    │   ├── operation_log.go      # 操作日志记录
    │   ├── permission.go         # RBAC权限校验
    │   ├── recovery.go           # 异常恢复
    │   ├── timeout.go            # 请求超时控制
    │   └── trace.go              # 链路追踪
    │
    ├── pkg/                       # 可复用基础设施包
    │   ├── auth/                 # 会话鉴权公共工具（TokenHash/AdminTokenKey/RequireAuth 泛型/会话写入 helper）
    │   ├── cache/                # 缓存管理（Redis/BigCache）
    │   ├── captcha/              # 验证码模块
    │   ├── configsync/           # 配置热同步
    │   ├── database/             # 数据库健康检查
    │   ├── errorx/               # 错误码定义
    │   ├── jwt/                  # JWT工具（含 TokenVersion 版本号机制）
    │   ├── migration/            # 数据迁移
    │   ├── password/             # 密码加密 + 强度校验（ValidateStrength）
    │   ├── ratelimit/            # 限流
    │   ├── redis/                # Redis封装
    │   ├── response/             # 统一响应封装
    │   ├── sentry/               # 错误追踪（sentry-go）
    │   ├── storage/              # 对象存储驱动（含 BuildPublicURL/CompleteUploadFromParams）
    │   ├── task/                 # 任务调度引擎
    │   └── utils/                # 通用工具（含 NormalizePaging/GetIntWithDefault/NewSecretToken/TokenHash）
    │
    ├── repository/                # 数据访问层
    │   ├── content/              # 内容管理仓储
    │   ├── log/                  # 日志仓储
    │   ├── open_platform/        # 开放平台仓储（App、OpenLog）
    │   ├── storage/              # 存储仓储
    │   └── system/               # 系统管理仓储
    │
    └── service/                   # 业务服务层
        ├── content/              # 内容管理服务
        ├── log/                  # 日志服务
        ├── open_platform/        # 开放平台服务（AppService含GetAppStorageDriver）
        ├── storage/              # 存储服务（RecordService含应用存储配置解析）
        └── system/               # 系统管理服务
	```

### 2.1 核心依赖

| 依赖 | 用途 |
|------|------|
| gin-gonic/gin | HTTP 框架 |
| gin-contrib/cors | 跨域中间件 |
| hellofresh/health-go | 健康检查 |
| golang-migrate/migrate | 数据库迁移 |
| minio-go | 对象存储 SDK |
| ulule/limiter | 限流 |
| redis/go-redis | Redis 客户端 |
| gorm.io/gorm | ORM 框架 |
| google/wire | 依赖注入 |

---

## 三、分层调用链

```
┌─────────────────────────────────────────────────────────────┐
│  HTTP Request                                               │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  Router (interface/admin/http/router)                       │
│  - 路由注册与分组                                           │
│  - 中间件挂载（JWT/RBAC/日志/超时）                         │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  Handler (interface/admin/http/handler)                     │
│  - 参数绑定（BindJSON/Query/Uri）                           │
│  - 参数校验                                                 │
│  - 调用Service                                              │
│  - 返回统一响应                                             │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  Service (service/)                                         │
│  - 业务规则实现                                             │
│  - 多仓储聚合                                               │
│  - 缓存/配置联动                                            │
│  - 禁止出现*gin.Context                                     │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  Repository (repository/)                                   │
│  - CRUD操作                                                 │
│  - 查询拼装（GORM）                                         │
│  - 所有 repo 调用通过 getDB(ctx) 统一获取 *gorm.DB          │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  Entity (domain/entity)                                     │
│  - 数据库实体定义                                           │
│  - GORM标签映射                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 四、二次开发指南

完整的二次开发示例（以**评论管理**模块为例，覆盖 Entity → Repository → DTO → Service → Handler → Router → Wire 全流程）已移到独立文档：

> 👉 **[二次开发指南](development-guide.md)**

包含：
- 详细代码示例（每个层级都附带红线规范说明）
- TransactionManager 事务指南（TM 架构图、标准范式、DeleteBatch fail-closed、注入方式）
- DTO/Entity 隔离规范（为什么需要隔离、Update 的 GetByID+patch+Save 模式、BFF userBase 模式）
- 常见踩坑记录

---

## 五、关键规范

### 5.1 错误处理规范

- 使用 `internal/pkg/errorx` 中定义的错误码
- Handler层统一使用 `response.Error()` 或 `response.Success()` 返回
- Service层返回具体的业务错误，不处理HTTP响应

### 5.2 事务处理规范

NetyAdmin 使用 `TransactionManager`（TM）统一管理数据库事务。TM 通过 context 隐式传递事务句柄，Repository 层通过 `getDB(ctx)` 自动区分是否在事务中。

#### TM 架构

```
┌───────────────────────────────────────────────────────┐
│  TransactionManager（无状态单例，DI 复用）              │
│                                                        │
│  Begin(ctx) → (txCtx, tx)  开启事务，注入 context      │
│  Commit(tx)  → 提交 + 执行 AfterCommit 钩子            │
│  Rollback(tx) → 回滚                                   │
│  AfterCommit(tx, func()) → 注册提交后回调              │
└───────────────────────────────────────────────────────┘
                             │
                             ▼
┌───────────────────────────────────────────────────────┐
│  Repository 通过 getDB(ctx) 统一取 *gorm.DB            │
│  - ctx 中有事务 → 返回 tx.DB（落入事务）               │
│  - ctx 中无事务 → 返回连接池（正常 CRUD）              │
└───────────────────────────────────────────────────────┘
```

#### 红线规则

| 规则 | 说明 |
|------|------|
| **Repository 不自管事务** | 禁止使用 `r.db.Transaction(func(tx){...})` 或 `r.db.WithContext(ctx).Transaction(...)` |
| **多步写操作必须用 TM** | 两个以上 repo 调用必须用 `tm.Begin → tm.Commit/Rollback` |
| **所有 repo 调用传 txCtx** | 事务内的 repo 调用必须用 `txCtx`（Begin 返回的 context），不是原始 `ctx` |
| **缓存失效在 Commit 之后** | 缓存失效 / 事件发布必须在 `tm.Commit()` 成功之后执行 |
| **Redis 处理在事务前** | `clearLoginLockCache` 等 Redis 操作在事务前调用，不进事务 |
| **fail-closed** | 失败直接 return error，不尝试补偿 |

#### TM 标准范式

**单事务多步写：**

```go
func (s *xxxService) MultiStepOp(ctx context.Context, args) error {
    // 事务前：预校验（用原始 ctx）
    old, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return errorx.New(errorx.CodeNotFound, "资源不存在")
    }

    // 事务前：Redis 操作（不进事务）
    s.clearLoginLockCache(ctx, id)

    // TM 单事务
    txCtx, tx := s.tm.Begin(ctx)

    // 第一步（用 txCtx！）
    if err := s.repo.DoA(txCtx, id); err != nil {
        slog.Error("...", "err", err)
        s.tm.Rollback(tx)
        return errorx.New(errorx.CodeInternalError, "操作A失败")
    }
    // 第二步（用 txCtx！）
    if err := s.repo.DoB(txCtx, id); err != nil {
        slog.Error("...", "err", err)
        s.tm.Rollback(tx)
        return errorx.New(errorx.CodeInternalError, "操作B失败")
    }
    // 提交
    if err := s.tm.Commit(tx); err != nil {
        slog.Error("commit failed", "err", err)
        return errorx.New(errorx.CodeInternalError, "事务提交失败")
    }

    // Commit 成功后：失效缓存（用原始 ctx，不是 txCtx）
    if cErr := s.cacheMgr.InvalidateByTags(ctx, tag); cErr != nil {
        slog.Warn("cache invalidation failed", "err", cErr)
    }
    return nil
}
```

**DeleteBatch fail-closed 范式：**

```go
func (s *xxxService) DeleteBatch(ctx context.Context, ids []string) error {
    var skipped []string
    for _, id := range ids {
        // 业务规则拒绝：不存在的 id 跳过，不阻断
        if _, err := s.repo.GetByID(ctx, id); err != nil {
            skipped = append(skipped, fmt.Sprintf("id %s：不存在", id))
            continue
        }
        // 事务前清理缓存
        s.clearLoginLockCache(ctx, id)

        txCtx, tx := s.tm.Begin(ctx)
        if err := s.repo.IncrementTokenVersion(txCtx, id); err != nil {
            slog.Error("...", "err", err)
            s.tm.Rollback(tx)
            return errorx.New(errorx.CodeInternalError, fmt.Sprintf("id %s 处理失败", id)) // 事务失败：立即返回
        }
        if err := s.repo.Delete(txCtx, id); err != nil {
            slog.Error("...", "err", err)
            s.tm.Rollback(tx)
            return errorx.New(errorx.CodeInternalError, fmt.Sprintf("id %s 处理失败", id))
        }
        if err := s.tm.Commit(tx); err != nil {
            slog.Error("commit failed", "err", err)
            return errorx.New(errorx.CodeInternalError, fmt.Sprintf("id %s 处理失败", id))
        }
        // Commit 后失效缓存
    }
    if len(skipped) > 0 {
        return errorx.New(errorx.CodeForbidden, fmt.Sprintf("部分用户被跳过：%s", strings.Join(skipped, "; ")))
    }
    return nil
}
```

**Repository getDB 实现：**

```go
// 每个 Repository 都通过 getDB 统一取 *gorm.DB
func (r *recordRepo) getDB(ctx context.Context) *gorm.DB {
    return database.GetDB(ctx, r.db) // GetDB 自动判断 ctx 中是否有事务
}

// Repository 内部方法都调 getDB，不直接使用 r.db
func (r *recordRepo) LockRecordByID(ctx context.Context, id uint) (*Record, error) {
    var record Record
    err := r.getDB(ctx).Set("gorm:query_option", "FOR UPDATE").First(&record, id).Error
    return &record, err
}
```

#### TM 注入方式

```go
// 1. wire.go 中的 tm 实例在 Bootstrap 中创建
tm := database.NewTransactionManager(db)

// 2. 传给 initServices，由 initServices 注入到各个 service
s.someService = NewSomeService(someRepo, tm)

// 3. Service struct 声明 tm 字段
type someService struct {
    repo someRepo.Repository
    tm   *database.TransactionManager
}
```

### 5.3 DTO/Entity 隔离规范

**为什么需要隔离？**
- Handler 只做协议转换（参数绑定 + 调 Service + 统一响应），不应知道 entity 结构
- Service 层通过 DTO 明确入参边界，不受 entity 字段变化影响
- Admin/Client 两端的入参完全不同（如 client 登录需要 platform，admin 不需要），统一 DTO 会导致混乱

#### 规范细则

**DTO 定义规则：**
- DTO 只含业务字段，不含 `ID`/`CreatedAt`/`UpdatedAt`/`DeletedAt`/`Password` 等持久化字段
- `CreateXxxReq` 包含创建所需的所有字段（不含 ID）
- `UpdateXxxReq` 只含可修改的业务字段（ID 由 URL `:id` 传入，不放在 body 中）
- Admin 端 DTO 放在 `internal/interface/admin/dto/`，Client 端 DTO 放在 `internal/interface/client/dto/v1/`
- 两端 DTO **禁止跨端 import**（如 client handler 不可 import admin DTO，反之亦然）

**Service 接口签名规则：**
- `Create(ctx, req *dto.CreateXxxReq) error`
- `Update(ctx, id uint64, req *dto.UpdateXxxReq) error`
- Service 内部构造 entity 再调 repo，Handler 不直接传 entity 给 Service

**Handler 改造规则：**
- 禁止 import `domain/entity/` 包
- 禁止直接调用 cacheMgr / repository（应通过 Service 层完成）
- Update 的 ID 从 `c.Param("id")` 解析，不在 body 中

#### Update 实现：GetByID + patch + Save

**背景**：GORM 的 `Save` 是全字段更新，如果 DTO 字段少于 entity，零值会覆盖数据库已有值（如 `CreatedAt`/`DeletedAt` 被零值覆盖）。

**解决方案**：
```go
func (s *xxxService) Update(ctx context.Context, id uint64, req *dto.UpdateXxxReq) error {
    // 1. 先 GetByID 取旧 entity（保留 ID/CreatedAt/DeletedAt 不被覆盖）
    old, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return errorx.New(errorx.CodeNotFound)
    }

    // 2. 唯一性校验（排除自身）
    if req.Name != "" && req.Name != old.Name {
        exists, _ := s.repo.ExistsByName(ctx, req.Name, id)
        if exists {
            return errorx.New(errorx.CodeAlreadyExists, "名称已存在")
        }
        old.Name = req.Name
    }

    // 3. patch 业务字段（不动 ID/CreatedAt/DeletedAt）
    if req.Phone != "" {
        old.Phone = req.Phone
    }

    // 4. Save（或 Update）
    return s.repo.Save(ctx, old)
}
```

#### BFF Service 端隔离（user 模块参考）

当 Admin 和 Client 两端的业务逻辑差异大但共享底层依赖（repo、jwt、cache、TM）时，可采用 **userBase 共享底层 + 独立接口** 模式：

```go
// userBase 封装共享依赖和横切方法
type userBase struct {
    repo       userRepo.UserRepository
    cacheMgr   cache.LazyCacheManager
    tm         *database.TransactionManager
    // ... 更多共享依赖
}

func (b *userBase) validatePasswordStrength(ctx context.Context, password string) error { ... }
func (b *userBase) clearLoginLockCache(ctx context.Context, userID string) { ... }

// Admin 端 service：仅 import admin/dto/user
type UserAdminService interface {
    Create(ctx context.Context, req *adminDto.CreateUserReq) error
    Update(ctx context.Context, id string, req *adminDto.UpdateUserReq) error
}
type userAdminService struct { userBase }
func NewUserAdminService(base userBase) UserAdminService {
    return &userAdminService{userBase: base}
}

// Client 端 service：仅 import client/dto/v1
type UserClientService interface {
    Register(ctx context.Context, req *clientDto.UserRegisterReq) error
    DeleteAccount(ctx context.Context, userID string) error
}
type userClientService struct { userBase }
func NewUserClientService(base userBase) UserClientService {
    return &userClientService{userBase: base}
}

// wire.go 注入
userBase := userService.NewUserBase(...)
s.userAdmin = userService.NewUserAdminService(userBase)
s.userClient = userService.NewUserClientService(userBase)
```

---

## 六、测试规范

### 6.1 框架与断言

| 规范 | 说明 |
|------|------|
| 测试框架 | `github.com/stretchr/testify`（主要使用 `assert`） |
| 文件命名 | `xxx_test.go`，与被测文件同目录同包（外部测试包 `xxx_test`） |
| 测试风格 | **表驱动测试**（table-driven tests），用 `[]struct{name; input; want}` 组织用例 |
| 子测试 | 使用 `t.Run(tt.name, func(t *testing.T) {...})` |
| 断言 | `assert.Equal(t, expected, actual)`、`assert.True/False`、`assert.NoError` |

### 6.2 代码示例

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

### 6.3 运行命令

```bash
cd server
go test ./...                          # 全部测试
go test ./internal/pkg/errorx/... -v   # 指定包，详细输出
```

参考实现：`internal/pkg/errorx/errorx_test.go`

---

## 七、Swagger / API 文档

### 7.1 Handler注解规范

每个Handler方法**必须**编写标准Swagger注解：

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

### 7.2 全局元信息

在 `cmd/server/main.go` 中定义全局Swagger元信息（`@title`、`@version`、`@BasePath` 等）。

### 7.3 生成与访问

```bash
cd server
swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
```

- 生成产物：`server/docs/`（`docs.go`、`swagger.json`、`swagger.yaml`）
- 访问地址：`http://localhost:9090/swagger/index.html`（仅 `debug` 模式开放）
- 路由注册：`internal/interface/admin/http/router/router.go`

---

## 八、相关文档

- [状态码规范](./status-codes.md)
- [API管理指南](./api-management.md)
- [Sentry错误追踪](./server-module-sentry.md)
- [缓存模块详解](./server-module-cache.md)
- [验证码模块详解](./server-module-captcha.md)
- [任务系统详解](./server-module-task.md)
- [字典模块详解](./server-module-dict.md)
- [内容管理模块详解](./server-module-content.md)
- [存储模块详解](./server-module-storage.md)
- [日志模块详解](./server-module-log.md)
- [数据迁移详解](./server-module-migration.md)
