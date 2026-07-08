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
    │   ├── jwt/                  # JWT工具（RS256 非对称签名 + TokenVersion 版本号机制 + alg confusion 防御）
    │   ├── mask/                 # 敏感字段脱敏（导出 SensitiveFieldKeys 单一事实源，供 operation_log 等模块引用）
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
        └── system/               # 系统管理服务（含 CaptchaService 验证码生成/校验，位于 service/system/captcha.go）
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
│  Commit(tx)  → 提交                                    │
│  Rollback(tx) → 回滚                                   │
│  WithTransaction(ctx, fn) → 闭包事务（推荐）            │
│    自动处理 panic/error 路径的 Rollback                │
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
| **Repository 不返回业务错误** | 禁止 `errorx.New(...)`，只返回原始 GORM 错误（如 `gorm.ErrRecordNotFound`）；业务错误码映射（`CodeNotFound`/`CodeInternalError` 等）由 Service 层通过 `errors.Is(err, gorm.ErrRecordNotFound)` 完成 |
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
    if cErr := s.cacheFast.InvalidateByTags(ctx, tag); cErr != nil {
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

### 5.3 可信代理配置（防止 IPAC 绕过）

Gin 默认信任所有代理，`c.ClientIP()` 会取 `X-Forwarded-For` 头的第一个值。攻击者只需在请求头中伪造 `X-Forwarded-For: 1.2.3.4` 即可让 `c.ClientIP()` 返回伪造的 IP，从而绕过 IPAC 的 IP 黑 / 白名单。

**强制要求**：server 启动时必须调用 `r.SetTrustedProxies(cfg.Server.TrustedProxies)` 配置可信代理。

```go
// cmd/server/main.go 或 app/init.go
r := gin.New()
r.SetTrustedProxies(cfg.Server.TrustedProxies)  // 必须在路由注册前调用
```

**配置项**（`config.toml`）：

```toml
[server]
# 可信代理 IP/CIDR 列表，默认空数组 = 不信任任何代理
trusted_proxies = []
# 生产环境部署在 Nginx/CDN 后须填写真实代理 IP/CIDR，例如：
# trusted_proxies = ["127.0.0.1", "10.0.0.0/8", "172.16.0.0/12"]
```

**行为**：

- 默认空数组 `[]` = 不信任任何代理 → `c.ClientIP()` 回退到 `RemoteAddr`（TCP 真实源 IP）
- 配置后，仅当请求直接来自可信代理 IP 时，才会解析 `X-Forwarded-For`；否则回退到 `RemoteAddr`
- IPAC 中间件依赖 `c.ClientIP()` 取客户端 IP，必须保证此值可信

### 5.4 DTO/Entity 隔离规范

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
- 禁止直接调用 cacheFast / cacheSlow / repository（应通过 Service 层完成）
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
    cacheSlow  cache.SecurityCache
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
go test ./internal/service/... -v      # 业务层测试（含 admin_auth / user_auth / menu / storage 等）
```

参考实现：`internal/pkg/errorx/errorx_test.go`

### 6.4 覆盖率门禁

Service 层覆盖率门禁：**≥ 70%**。

```bash
cd server
bash scripts/test-coverage.sh
```

脚本执行 `go test -coverprofile=cover.out -coverpkg=./... ./...`，生成：

| 产物 | 说明 |
|------|------|
| `cover.out` | Go 覆盖率 profile，可被 coveralls / codecov 消费 |
| `cover.html` | 可读 HTML 覆盖率报告，便于本地查看 |

门禁规则：
- Service 层（`internal/service/...`）平均覆盖率 **≥ 70%**，未达标脚本以非零退出码退出
- 新增 Service 方法必须配套单元测试（参考 `admin_auth_test.go` / `user_auth_test.go` / `menu_test.go` / `record_test.go`）
- Mock 策略：手写 mock 结构体实现接口（项目无 `testify/mock` 依赖）；TM 依赖场景用 sqlite in-memory 支撑 `Begin/Commit/Rollback`

#### Repository 集成测试（TODO）

当前基线仅覆盖 Service 层单元测试。Repository 集成测试基础设施（`testcontainers-go` + PostgreSQL）暂未引入，后续可按需补充：

```
TODO: 引入 github.com/testcontainers/testcontainers-go
TODO: 封装 TestRepo helper：启动 PostgreSQL 容器 + AutoMigrate + 返回 *gorm.DB
TODO: 至少 1 个 Repository 集成测试样例（如 repository/system/admin_test.go）
```

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

---

## 12-Factor 配置

NetyAdmin 遵循 [12-Factor App](https://12factor.net/) 配置原则：**配置存储在环境变量中**，与代码分离，便于跨环境部署与密钥轮换。

### 配置优先级

加载顺序：**环境变量 > TOML > 零值**。

`config.Load` 内部先用 TOML 反序列化 `config.toml`，再通过 `reflect` 遍历 `Config` 结构体，对带 `env:"NETYADMIN_XXX"` 标签的字段用环境变量覆盖。环境变量未设置时保留 TOML 值；显式设置为空字符串时覆盖为空（`os.LookupEnv` 语义）。

> 仅叶子字段支持 env 覆盖；`map` 类型字段（如 `ignore_transactions`、`task.jobs`）不参与覆盖，仍由 TOML 配置。`[]string` 类型字段（如 `[cors].allowed_origins`）支持逗号分隔的 env 覆盖（空字符串 = 空切片，符合 fail-closed 语义）。

### 支持环境变量覆盖的字段

| 字段 | 环境变量 | 说明 |
|------|---------|------|
| `[server].port` | `NETYADMIN_SERVER_PORT` | HTTP 端口 |
| `[server].mode` | `NETYADMIN_SERVER_MODE` | `debug` / `release`（生产） |
| `[server].handler_timeout` | `NETYADMIN_SERVER_HANDLER_TIMEOUT` | 请求处理超时（http.TimeoutHandler），默认 25s，应略小于 read/write_timeout |
| `[server].shutdown_timeout` | `NETYADMIN_SERVER_SHUTDOWN_TIMEOUT` | 优雅关闭超时（srv.Shutdown 等待时长），默认 30s |
| `[database].host` | `NETYADMIN_DB_HOST` | PostgreSQL 主机 |
| `[database].port` | `NETYADMIN_DB_PORT` | PostgreSQL 端口 |
| `[database].user` | `NETYADMIN_DB_USER` | PostgreSQL 用户名 |
| `[database].password` | `NETYADMIN_DB_PASSWORD` | PostgreSQL 密码（**敏感**） |
| `[database].dbname` | `NETYADMIN_DB_NAME` | PostgreSQL 库名 |
| `[database].sslmode` | `NETYADMIN_DB_SSLMODE` | SSL 模式 |
| `[redis].host` | `NETYADMIN_REDIS_HOST` | Redis 主机 |
| `[redis].port` | `NETYADMIN_REDIS_PORT` | Redis 端口 |
| `[redis].password` | `NETYADMIN_REDIS_PASSWORD` | Redis 密码（**敏感**） |
| `[jwt].private_key_pem` | `NETYADMIN_JWT_PRIVATE_KEY_PEM` | JWT RS256 私钥 PEM 内容（**敏感**，与 `private_key_file` 二选一） |
| `[jwt].public_key_pem` | `NETYADMIN_JWT_PUBLIC_KEY_PEM` | JWT RS256 公钥 PEM 内容（与 `public_key_file` 二选一） |
| `[jwt].access_token_ttl` | `NETYADMIN_JWT_ACCESS_TOKEN_TTL` | Access Token 有效期，默认 30m，必须 ≤ 30 分钟 |
| `[jwt].refresh_token_ttl` | `NETYADMIN_JWT_REFRESH_TOKEN_TTL` | Refresh Token 有效期，默认 168h（7 天） |
| `[security].aes_key` | `NETYADMIN_AES_KEY` | AES 加解密密钥（**敏感**） |
| `[security].upload_hmac_key` | `NETYADMIN_UPLOAD_HMAC_KEY` | 存储上传 HMAC 密钥（**敏感**，原 `[jwt].secret` 复用职责独立化） |
| `[email].password` | `NETYADMIN_EMAIL_PASSWORD` | SMTP 密码（**敏感**） |
| `[sms].secret_id` | `NETYADMIN_SMS_SECRET_ID` | SMS SecretID（**敏感**） |
| `[sms].secret_key` | `NETYADMIN_SMS_SECRET_KEY` | SMS SecretKey（**敏感**） |
| `[bus].driver` | `NETYADMIN_BUS_DRIVER` | 事件总线驱动（`redis` / `memory`） |
| `[sentry].dsn` | `NETYADMIN_SENTRY_DSN` | Sentry DSN |
| `[sentry].environment` | `NETYADMIN_SENTRY_ENVIRONMENT` | Sentry 环境标识 |
| `[sentry].release` | `NETYADMIN_SENTRY_RELEASE` | Sentry 版本号 |
| `[cors].allowed_origins` | `NETYADMIN_CORS_ALLOWED_ORIGINS` | CORS 白名单（逗号分隔） |
| `[security_headers].csp` | `NETYADMIN_SECURITY_HEADERS_CSP` | Content-Security-Policy 头内容 |

### 仓库配置文件

- 仓库提交的配置文件为 **`server/config.example.toml`**（模板），所有敏感值已替换为 `<CHANGE_ME_IN_PRODUCTION>` 占位符。
- **运行时不直接读取 `config.example.toml`**：部署时需先复制为 `config.toml` 并填入真实值，或仅通过环境变量覆盖。
- 复制命令：
  ```bash
  cp server/config.example.toml server/config.toml
  ```
- 本地开发：直接编辑 `config.toml`（建议加入 `.gitignore`，避免误提交真实密钥）。
- 容器化部署：仅注入环境变量，`config.toml` 可保留占位符（启动期 `ValidateConfig` 会校验最终值）。

### 启动期强校验（`ValidateConfig`）

`cmd/server/main.go` 在 `config.Load` 之后、`InitDB` 之前调用 `config.ValidateConfig(cfg)`：

- `mode == "debug"` 时**跳过**校验（开发环境允许使用默认值便于快速启动）。
- `mode != "debug"`（生产 / 预发布）时，以下字段不得为默认值或 `<CHANGE_ME_IN_PRODUCTION>` 占位符，否则 `log.Fatal` 拒绝启动：

  | 字段 | 禁止值 |
  |------|--------|
  | `[database].password` | `123456` / `<CHANGE_ME_IN_PRODUCTION>` |
  | `[jwt].private_key_pem` / `[jwt].private_key_file` | 空 / `<CHANGE_ME_IN_PRODUCTION>`（RS256 必须提供有效私钥） |
  | `[jwt].public_key_pem` / `[jwt].public_key_file` | 空 / `<CHANGE_ME_IN_PRODUCTION>`（RS256 必须提供有效公钥） |
  | `[security].aes_key` | `netyadmin-aes-key-32-chars-long!` / `<CHANGE_ME_IN_PRODUCTION>` |
  | `[security].upload_hmac_key` | 空 / `<CHANGE_ME_IN_PRODUCTION>`（storage HMAC 密钥独立化后必须显式配置） |
  | `[email].password`（仅 `email.enabled = true`） | `your-password` / `<CHANGE_ME_IN_PRODUCTION>` |

### 生产部署 Checklist

1. `cp config.example.toml config.toml`，或仅依赖环境变量（推荐）。
2. 设置 `[server].mode = "release"`（或 `NETYADMIN_SERVER_MODE=release`）。
3. 通过环境变量注入所有敏感值：
   ```bash
   export NETYADMIN_DB_PASSWORD='强密码'
   export NETYADMIN_JWT_PRIVATE_KEY_PEM='-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----'
   export NETYADMIN_JWT_PUBLIC_KEY_PEM='-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----'
   export NETYADMIN_AES_KEY='随机32字节密钥'
   export NETYADMIN_UPLOAD_HMAC_KEY='随机32字节HMAC密钥'
   export NETYADMIN_REDIS_PASSWORD='Redis密码'
   export NETYADMIN_EMAIL_PASSWORD='SMTP密码'      # 若启用邮件
   export NETYADMIN_SMS_SECRET_ID='SMS SecretID'   # 若启用短信
   export NETYADMIN_SMS_SECRET_KEY='SMS SecretKey'
   ```
4. 启动服务：`./server`。若任一敏感值仍为占位符，进程会 `log.Fatal` 退出。
5. 验证：日志中出现「服务器启动」即表示配置校验通过。

---

## CORS 跨域配置

NetyAdmin 通过 `[cors]` 配置项实现 **Origin 白名单** 跨域策略，使用 `github.com/gin-contrib/cors` 实现。

### 配置项

```toml
[cors]
# 允许跨域的来源白名单（精确匹配，不支持通配符）
# 空数组 = 拒绝所有跨域请求（fail-closed）
allowed_origins = ["http://localhost:5173"]
```

环境变量覆盖：

```bash
export NETYADMIN_CORS_ALLOWED_ORIGINS='https://admin.example.com,https://app.example.com'
```

### 安全策略

- **精确匹配白名单**：原 `AllowOriginFunc: func(origin string) bool { return true }` 的反射行为已废弃，存在 CSRF 与 Cookie 泄露风险（任意站点可携带 Cookie 访问后端 API）。
- **空白名单 fail-closed**：未配置 `allowed_origins` 时拒绝所有跨域请求，强制运维显式列出可信来源。
- **AllowCredentials: true**：仅对白名单内的 Origin 生效，允许携带 Cookie。该字段不能与 `AllowAllOrigins=true` 同时使用（库会 panic），改用 `AllowOriginFunc` 做白名单校验是库官方推荐的安全等价表达。
- **MaxAge = 24h**：预检结果缓存 24 小时，减少 OPTIONS 请求频率。
- **AllowMethods / AllowHeaders**：覆盖 RESTful 全部写读语义，与原自研实现保持一致。

### 中间件链顺序

```
RequestID → CORS → SecurityHeaders → Recovery → sentrygin → ...
```

参见 [RULES.md §六](../RULES.md) 中间件链顺序。

---

## 安全响应头（Security Headers）

NetyAdmin 通过 `middleware.SecurityHeaders` 中间件设置一组安全响应头，缓解常见的 Web 攻击向量。

### 配置项

```toml
[security_headers]
# Content-Security-Policy 头内容（为空时不设置该头）
csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'"
```

环境变量覆盖：

```bash
export NETYADMIN_SECURITY_HEADERS_CSP="default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'"
```

### 响应头清单

| 响应头 | 值 | 说明 |
|--------|-----|------|
| `X-Content-Type-Options` | `nosniff` | 防 MIME 嗅探 |
| `X-Frame-Options` | `DENY` | 防点击劫持（同源 deny） |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | 控制 Referer 头泄露 |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` | HSTS，仅 HTTPS 时下发，强制浏览器 1 年内使用 HTTPS |
| `Content-Security-Policy` | 可配置（默认 `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'`） | CSP 防 XSS 注入 |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | 禁用敏感浏览器能力 |

### 设计要点

- **HSTS 仅 HTTPS 下发**：`Strict-Transport-Security` 在 `c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"` 时设置，避免 HTTP 降级场景下被中间人拦截后变成不可逆的 HSTS 锁定。新增 `X-Forwarded-Proto == "https"` 分支用于反向代理终止 TLS 场景（Nginx/CDN 转发 HTTP 到应用，但需下发 HSTS 给浏览器）。**信任前提**：`X-Forwarded-Proto` 可信的前提是 `[server].trusted_proxies` 已配置真实代理 CIDR（参见 [server].trusted_proxies），否则攻击者可伪造该 header 触发 HSTS 锁定攻击。直连无代理时 `trusted_proxies = []`，行为退化为仅 `TLS != nil` 时下发。
- **CSP 可配置**：默认值允许同源脚本与样式（`'unsafe-inline'` 用于 Vue 注入的内联样式）。生产环境建议进一步收紧：移除 `'unsafe-inline'`，改用 nonce 或 hash。`csp` 为空时不设置该头，保持向后兼容性。
- **已移除 X-XSS-Protection**：现代浏览器（Chrome 78+ / Edge / Firefox）已弃用并移除该过滤器，启用反而可能引入 XSS 攻击面（参见 [MDN](https://developer.mozilla.org/docs/Web/HTTP/Headers/X-XSS-Protection)）。防御 XSS 应使用 Content-Security-Policy。

---

## 优雅关闭（Graceful Shutdown）

NetyAdmin 实现了多阶段优雅关闭流程，确保在收到 `SIGINT` / `SIGTERM` 信号后，在途请求能被处理完成、内部缓冲能被刷盘、数据库连接能被正确释放，避免数据丢失与半完成状态。

### 关闭序列

```
SIGINT / SIGTERM
       │
       ▼
1. 活跃事务检查（slog.Error 告警）
       │
       ▼
2. srv.Shutdown(ctx)        ← 等待在途 HTTP 请求完成（最多 ShutdownTimeout）
       │
       ▼
3. dbHealthChecker.Stop()  ← 停止健康检查探活
       │
       ▼
4. taskManager.Stop()       ← 停止 cron 调度 + 等待 worker 退出（5s drain 超时）
       │
       ▼
5. logBus.Stop()            ← flush 所有日志桶到 DB（5s drain 超时）
       │
       ▼
6. eventBus.Close()        ← 关闭 Redis 订阅 goroutine
       │
       ▼
7. sqlDB.Close()            ← 关闭数据库连接池（必须在 task/logBus drain 之后）
       │
       ▼
8. Sentry.Flush(2s)        ← 刷盘未发送的 Sentry 事件
       │
       ▼
进程退出
```

### 配置项

```toml
[server]
# 优雅关闭超时（srv.Shutdown 等待在途请求的最大时长）。
# 零值 = 默认 30s。环境变量：NETYADMIN_SERVER_SHUTDOWN_TIMEOUT
shutdown_timeout = "30s"
```

环境变量覆盖：

```bash
export NETYADMIN_SERVER_SHUTDOWN_TIMEOUT=45s
```

### 关键设计要点

#### 1. 活跃事务计数器（TM ActiveTransactions）

`TransactionManager` 内部维护一个 `atomic.Int64` 活跃事务计数器：

- `Begin()` 递增
- `Commit()` / `Rollback()` 递减
- `ActiveTransactions() int64` 公开查询接口

优雅关闭时若 `ActiveTransactions() > 0`，会 `slog.Error` 告警并记录数量。这些未提交事务在 `srv.Shutdown` 等待在途请求退出时可能被强制中断，导致数据丢失。运维人员应关注此告警，排查是否有长事务阻塞或事务未正确 Commit/Rollback。

```go
// internal/pkg/database/tx_manager.go
type TransactionManager struct {
    db            *gorm.DB
    activeTxCount atomic.Int64
}

func (tm *TransactionManager) Begin(ctx context.Context) (context.Context, *Tx) {
    db := tm.db.WithContext(ctx).Begin()
    tx := &Tx{DB: db}
    tm.activeTxCount.Add(1)  // 递增
    return context.WithValue(ctx, TxKey, tx), tx
}

func (tm *TransactionManager) ActiveTransactions() int64 {
    return tm.activeTxCount.Load()
}
```

#### 2. drain 超时保护（5s）

`taskManager.Stop()` 和 `logBus.Stop()` 内部都用 `wg.Wait()` 等待 worker 退出。若 worker 卡死（如外部依赖故障），会导致整个关闭流程卡死，最终被 K8s / systemd 的 `SIGKILL` 强杀。

`app.go` 用 `stopWithTimeout` 包装这两个调用，最多等待 5s：

```go
func (a *App) stopWithTimeout(name string, stopFn func()) {
    done := make(chan struct{})
    go func() {
        defer func() {
            if r := recover(); r != nil {
                slog.Error("drain panic", "component", name, "panic", r)
            }
            close(done)
        }()
        stopFn()
    }()
    select {
    case <-done:
        slog.Info("drain 完成", "component", name)
    case <-time.After(drainTimeout):  // 5s
        slog.Warn("drain 超时，放弃等待（进程退出时由 OS 回收 goroutine）",
            "component", name, "timeout", drainTimeout)
    }
}
```

超时后仅 `slog.Warn` 告警并继续后续关闭步骤，不阻塞进程退出。仍在执行的 goroutine 由 OS 在进程退出时回收。

#### 3. 数据库连接池关闭时机

`sqlDB.Close()` 必须在 `taskManager.Stop()` 和 `logBus.Stop()` 完成之后执行。原因：

- **taskManager 的 cron / interval 任务**可能在退出前最后执行一次，需要 DB 连接
- **logBus 的 flush worker** 需要把缓冲的日志批量写入 DB

若先关闭连接池，这些 drain 操作会因连接已关闭而失败，导致任务数据丢失或日志未刷盘。

```go
// 4. taskManager + logBus drain（带 5s 超时）
a.stopWithTimeout("taskManager", a.taskManager.Stop)
a.stopWithTimeout("logBus", a.logBus.Stop)

// 5. eventBus 关闭
_ = a.eventBus.Close()

// 6. 关闭数据库连接池（必须在 drain 之后）
if sqlDB, err := a.db.DB(); err == nil && sqlDB != nil {
    if err := sqlDB.Close(); err != nil {
        slog.Warn("关闭数据库连接池失败", "error", err)
    }
}
```

### 运维观测建议

- **`active_transactions > 0` 告警**：优雅关闭时有未提交事务，应排查长事务
- **`drain 超时` 告警**：taskManager / logBus drain 超过 5s 未完成，应排查 worker 卡死原因
- **`ShutdownTimeout` 调优**：若在途请求量大，可适当调大（如 60s）；若需要快速重启，可调小（如 10s）

---

## Timeout 中间件返回 503

NetyAdmin 用 `http.TimeoutHandler` 包装 gin engine，当请求处理超过阈值时返回 HTTP 503 + JSON 错误体，而非依赖连接层超时（客户端会收到空响应或连接重置）。

### 配置项

```toml
[server]
# 请求处理超时（http.TimeoutHandler 包装 engine）。
# 应略小于 read_timeout / write_timeout，确保超时时由中间件返回 503 + JSON 错误体，
# 而非连接层超时断开（客户端收到空响应 / 连接重置）。
# 零值 = 默认 25s。环境变量：NETYADMIN_SERVER_HANDLER_TIMEOUT
handler_timeout = "25s"
read_timeout = 120
write_timeout = 120
```

环境变量覆盖：

```bash
export NETYADMIN_SERVER_HANDLER_TIMEOUT=20s
```

### 超时响应

超时时返回：

```
HTTP/1.1 503 Service Unavailable
Content-Type: text/plain; charset=utf-8
X-Content-Type-Options: nosniff

{"code":"100011","msg":"请求超时"}
```

> 注：`http.TimeoutHandler` 由 stdlib 实现，固定设置 `Content-Type: text/plain`，无法自定义 header。客户端应按 body 内容（JSON 字符串）解析，而非依赖 content-type。

### 与原 context.WithTimeout 中间件的区别

| 项 | 原实现（context.WithTimeout） | 新实现（http.TimeoutHandler） |
|---|---|---|
| **超时后行为** | 仅向 ctx 注入 deadline，不拦截响应 | 在新 goroutine 执行底层 handler，超时主动写入 503 + body |
| **客户端收到** | 空响应 / 连接重置（连接层超时断开） | 503 + `{"code":"100011","msg":"请求超时"}` |
| **handler 是否感知** | 是（ctx.Done() 触发） | 否（底层 handler 在新 goroutine 继续执行，结果被丢弃） |
| **可观测性** | 客户端无法区分「服务挂了」与「请求超时」 | 客户端可明确识别为请求超时 |

### 与 ReadTimeout / WriteTimeout 的协调

`handler_timeout` 应略小于 `read_timeout` / `write_timeout`，确保：

1. `http.TimeoutHandler` 先于连接层超时触发
2. 503 错误体能完整写入响应（在连接断开之前）
3. 客户端收到结构化错误体而非空响应

默认值：`handler_timeout = 25s` < `read_timeout = write_timeout = 120s`，留有 95s 余量。

### 包装位置

`http.TimeoutHandler` 包装整个 gin engine，而非作为 gin 中间件。原因：

- gin 中间件无法主动终止已开始的 handler chain（`c.Next()` 后无法回滚）
- `http.TimeoutHandler` 在 stdlib 层用新 goroutine 隔离执行，超时时由 stdlib 主动写入响应

```go
// internal/app/app.go
wrappedHandler := middleware.WrapWithTimeout(a.engine, handlerTimeout)

srv := &http.Server{
    Handler: wrappedHandler,  // 包装后的 handler
    ...
}
```

```go
// internal/middleware/timeout.go
func WrapWithTimeout(handler http.Handler, timeout time.Duration) http.Handler {
    if timeout <= 0 {
        return handler  // 零值不启用超时
    }
    return http.TimeoutHandler(handler, timeout, TimedOutBody)
}
```

### 关于错误码 100011

`errorx.CodeRequestTimeout`（100011，"请求超时"）专用于 `http.TimeoutHandler` 触发的请求处理超时响应，与限流码 `CodeTooManyRequest`（100006，"请求过于频繁"）数值解耦：

- **限流路径**：`CodeTooManyRequest` (100006) + msg "请求过于频繁"，HTTP 429
- **超时路径**：`CodeRequestTimeout` (100011) + msg "请求超时"，HTTP 503

客户端可仅凭 `code` 字段区分两种语义（限流 vs 超时），无需依赖 HTTP 状态码或 msg 文案。详见 `docs/status-codes.md` 4.1 节。

---

## JWT 认证（RS256 非对称签名）

NetyAdmin 自 Round 4 审计起，JWT 签名算法由对称 HS256 改为非对称 RS256：私钥签发（服务端持有）+ 公钥验证（可下发至网关 / 微服务）。`pkg/jwt/jwt.go` 是 JWT 工具的唯一入口，集成 TokenVersion 版本号机制用于主动吊销。

### 配置项

```toml
[jwt]
# RS256 私钥（签发 token 用），二选一：文件路径或 PEM 内容
private_key_file = "/etc/netyadmin/rsa_private.pem"
# private_key_pem = "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"

# RS256 公钥（验证 token 用），二选一：文件路径或 PEM 内容
public_key_file = "/etc/netyadmin/rsa_public.pem"
# public_key_pem = "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----"

# Access Token 有效期，必须 ≤ 30 分钟（默认 30m）
access_token_ttl = "30m"

# Refresh Token 有效期，默认 7 天（168h）
refresh_token_ttl = "168h"

# Token 签发者
issuer = "netyadmin"
```

环境变量覆盖：

```bash
export NETYADMIN_JWT_PRIVATE_KEY_PEM='-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----'
export NETYADMIN_JWT_PUBLIC_KEY_PEM='-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----'
export NETYADMIN_JWT_ACCESS_TOKEN_TTL=30m
export NETYADMIN_JWT_REFRESH_TOKEN_TTL=168h
```

### alg confusion 防御

`ParseToken` 显式校验 `*jwt.SigningMethodRSA`，拒绝以下攻击：

- **HS256 攻击**：攻击者用公钥作为 HMAC secret 伪造 token。`ParseToken` 不接受 `SigningMethodHMAC`，直接返回错误。
- **none 攻击**：攻击者将 alg 改为 `none` 绕过签名校验。`ParseToken` 不接受 `SigningMethodNone`。

```go
// pkg/jwt/jwt.go
token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
    // 显式校验签名算法，拒绝 HS256 / none
    if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
    }
    return publicKey, nil
})
```

### Access / Refresh TTL 拆分

Round 4 将单一 `expiration=168h` 拆分为 `access_token_ttl` + `refresh_token_ttl`：

- **Access Token 短命**：默认 30 分钟，泄露窗口期短，配合 TokenVersion 可主动吊销。
- **Refresh Token 长命**：默认 7 天，避免用户频繁重新登录。
- **禁止 `*2` 模式**：原实现 RefreshToken TTL = `expiration * 2` 是隐式约定，已废弃。
- `expirationFor(tokenType)` 按 tokenType 返回独立 TTL。

### storage HMAC 密钥独立化

原 `cfg.JWT.Secret` 同时承担「JWT 签名」与「storage upload HMAC」双重职责，是密钥复用 hack。RS256 迁移后 JWT 不再需要 secret，storage HMAC 密钥独立为 `[security].upload_hmac_key`：

```toml
[security]
# 存储上传 HMAC 密钥（原 [jwt].secret 复用职责独立化）
upload_hmac_key = "<CHANGE_ME_IN_PRODUCTION>"
```

- 启动期 `ValidateConfig` fail-closed 校验：生产模式下为空 / 占位符时 `log.Fatal` 拒绝启动。
- 与 JWT 密钥解耦，任一泄露不影响另一功能。

### 相关红线

- [RULES.md §11.1](../RULES.md) JWT 签名算法
- [RULES.md §11.2](../RULES.md) Token TTL 拆分
- [SHARED.md §9.1](../SHARED.md) JWT HS256 → RS256 迁移
- [SHARED.md §9.2](../SHARED.md) Access / Refresh TTL 拆分

---

## 可选 TLS 配置

NetyAdmin 默认不启用应用层 TLS（由 Nginx / CDN 终止 TLS 是行业惯例）。Round 4 起新增可选 TLS 配置，用于单机部署无反向代理的场景。

### 配置项

```toml
[tls]
# 是否启用应用层 TLS（默认 false，由 Nginx 终止 TLS）
enable = false

# TLS 证书文件路径（enable = true 时必填）
cert_file = "/etc/netyadmin/server.crt"

# TLS 私钥文件路径（enable = true 时必填）
key_file = "/etc/netyadmin/server.key"
```

### 启用行为

`enable = true` 时，`app.go:Run` 同时启动两个监听：

1. **443 HTTPS**：主服务，加载 `cert_file` / `key_file` 提供 HTTPS 服务。
2. **80 → 443 跳转 goroutine**：监听 80 端口，所有请求返回 `301 Moved Permanently` 跳转到 HTTPS 对应路径。

```go
// internal/app/app.go
if cfg.TLS.Enable {
    // 主服务：443 HTTPS
    go func() {
        if err := srv.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil && err != http.ErrServerClosed {
            log.Fatalf("HTTPS listen: %v", err)
        }
    }()

    // 跳转服务：80 → 443
    go func() {
        redirectSrv := &http.Server{
            Addr: ":80",
            Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
            }),
        }
        if err := redirectSrv.ListenAndServe(); err != nil {
            log.Fatalf("HTTP redirect listen: %v", err)
        }
    }()
} else {
    // 默认：HTTP 模式（由 Nginx 终止 TLS）
    // ...
}
```

### 设计理由

- **默认关闭**：Nginx 终止 TLS 是行业惯例，证书管理 / HTTP/2 / OCSP stapling 在 Nginx 层更成熟。
- **职责分离**：应用层关注业务，TLS 终止由网关 / 反向代理负责。
- **本地开发友好**：默认关闭，开发者无需生成自签证书即可启动。
- **小规模部署可选开启**：单机部署无 Nginx 时，开启 `tls.enable = true` + 提供证书文件即可。

### 与 HSTS 的关系

启用 `tls.enable = true` 时，`c.Request.TLS != nil` 为真，HSTS 自动下发（参见 [安全响应头](#安全响应头security-headers)）。

若由 Nginx 终止 TLS（`tls.enable = false`），需配合 `X-Forwarded-Proto` 头让应用感知 HTTPS：

- Nginx 配置：`proxy_set_header X-Forwarded-Proto $scheme;`
- 应用配置：`[server].trusted_proxies = ["127.0.0.1"]`（信任 Nginx 转发）
- 此时 HSTS 下发条件 `c.GetHeader("X-Forwarded-Proto") == "https"` 满足。

---

## 敏感字段脱敏（pkg/mask）

NetyAdmin 通过 `pkg/mask` 包集中维护敏感字段列表，供操作日志、审计日志、Sentry 脱敏等场景统一引用，避免硬编码字段列表导致的不一致。

### 设计原则

- **单一事实源**：所有敏感字段在 `pkg/mask/fields.go` 一个文件维护，新增字段只需改此文件。
- **全小写 + 大小写不敏感匹配**：兼容 JSON tag 大小写差异（如 `appSecret` vs `app_secret`）。
- **可复用**：未来其他模块（如审计日志、Sentry 脱敏）可引用同一列表。
- **禁止硬编码字段列表**：禁止在中间件 / service / handler 本地维护一份 `[]string{"password", ...}` 切片。

### API

```go
// pkg/mask/fields.go

// SensitiveFieldKeys 导出敏感字段列表（全小写），供 operation_log 等模块引用。
// 新增敏感字段只需在此切片追加，无需修改调用方。
var SensitiveFieldKeys = []string{
    "password",
    "oldpassword",
    "newpassword",
    "appsecret",
    "app_key",
    "secret",
    "token",
    "access_token",
    "refresh_token",
    "api_key",
    "private_key",
    "client_secret",
    "session",
    "credential",
}

// IsSensitive 判断字段名是否敏感（大小写不敏感）。
func IsSensitive(field string) bool
```

### 使用示例

```go
// internal/middleware/operation_log.go
import "internal/pkg/mask"

func shouldMask(field string) bool {
    return mask.IsSensitive(field)  // 大小写不敏感匹配
}
```

### 相关红线

- [RULES.md §11.4](../RULES.md) 敏感字段脱敏集中化
- [SHARED.md §9.6](../SHARED.md) 敏感字段集中脱敏包 pkg/mask
