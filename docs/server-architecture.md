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
| **端隔离** | 按终端类型（admin/client）物理隔离接口层 |
| **分层清晰** | 严格遵循 `router -> handler -> service -> repository -> entity` 调用链 |
| **无侵入协议** | Service 层禁止出现 `*gin.Context`，只接收基础 Go 类型 |
| **DTO 专属** | 每个端的 DTO 独立存放，禁止全局共享 |
| **依赖注入** | 使用 Wire 进行依赖装配，便于测试和替换实现 |

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
├── migrations/                    # SQL迁移脚本
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
    │   │   ├── storage/          # 存储实体
    │   │   └── system/           # 系统管理实体
    │   │
    │   └── vo/                   # 面向前端的View Object
    │       ├── content/
    │       ├── log/
    │       └── system/
    │
    ├── interface/                 # 【接入层】按端隔离
    │   └── admin/                # 面向Admin-Web的接口
    │       ├── dto/              # Admin专用DTO
    │       │   ├── content/      # 内容管理DTO
    │       │   ├── log/          # 日志DTO
    │       │   ├── storage/      # 存储DTO
    │       │   └── system/       # 系统管理DTO
    │       │
    │       └── http/             # HTTP协议接入
    │           ├── handler/v1/   # Handler实现
    │           │   ├── admin/    # 管理员相关
    │           │   ├── auth/     # 认证相关
    │           │   ├── content/  # 内容管理
    │           │   ├── log/      # 日志管理
    │           │   ├── storage/  # 存储管理
    │           │   └── system/   # 系统管理
    │           │
    │           └── router/v1/    # 路由注册
    │               ├── admin.go
    │               ├── auth.go
    │               ├── content.go
    │               ├── log.go
    │               ├── router.go # 路由聚合入口
    │               ├── storage.go
    │               └── system.go
    │
    ├── job/                       # 内置任务
    │   ├── article_publish.go    # 文章定时发布
    │   ├── init.go               # 任务注册入口
    │   └── system_log_cleanup.go # 日志清理
    │
    ├── middleware/                # Gin中间件
    │   ├── auth.go               # JWT认证
    │   ├── operation_log.go      # 操作日志记录
    │   ├── permission.go         # RBAC权限校验
    │   ├── recovery.go           # 异常恢复
    │   ├── timeout.go            # 请求超时控制
    │   └── trace.go              # 链路追踪
    │
    ├── pkg/                       # 可复用基础设施包
    │   ├── cache/                # 缓存管理（Redis/BigCache）
    │   ├── captcha/              # 验证码模块
    │   ├── configsync/           # 配置热同步
    │   ├── database/             # 数据库健康检查
    │   ├── errorx/               # 错误码定义
    │   ├── jwt/                  # JWT工具
    │   ├── migration/            # 数据迁移
    │   ├── password/             # 密码加密
    │   ├── redis/                # Redis封装
    │   ├── response/             # 统一响应封装
    │   ├── storage/              # 对象存储驱动
    │   ├── task/                 # 任务调度引擎
    │   └── utils/                # 通用工具函数
    │
    ├── repository/                # 数据访问层
    │   ├── content/              # 内容管理仓储
    │   ├── log/                  # 日志仓储
    │   ├── storage/              # 存储仓储
    │   └── system/               # 系统管理仓储
    │
    └── service/                   # 业务服务层
        ├── content/              # 内容管理服务
        ├── log/                  # 日志服务
        ├── storage/              # 存储服务
        └── system/               # 系统管理服务
```

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
│  - 事务管理                                                 │
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

## 四、二次开发示例

### 4.1 新增业务模块（以"评论管理"为例）

#### 步骤1：定义实体

```go
// internal/domain/entity/content/comment.go
package content

import (
    "gorm.io/plugin/soft_delete"
)

type Comment struct {
    ID        uint                  `gorm:"primarykey"`
    ArticleID uint                  `gorm:"not null;index"`
    UserID    uint                  `gorm:"not null;index"`
    Content   string                `gorm:"type:text;not null"`
    Status    int8                  `gorm:"default:1"` // 1:正常 2:禁用
    CreatedAt int64                 `gorm:"autoCreateTime"`
    UpdatedAt int64                 `gorm:"autoUpdateTime"`
    DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli;default:0;index"`
}

func (Comment) TableName() string {
    return "comments"
}
```

> **注意**：NetyAdmin 使用 `soft_delete.DeletedAt`（BIGINT 类型）而非 `gorm.DeletedAt`（TIMESTAMP 类型），以支持毫秒级软删除时间戳，并避免时区问题。数据库列类型为 `BIGINT DEFAULT 0`。

#### 步骤2：创建仓储

```go
// internal/repository/content/comment.go
package content

import (
    "context"
    "server/internal/domain/entity/content"
    "gorm.io/gorm"
)

type CommentRepository interface {
    Create(ctx context.Context, comment *content.Comment) error
    GetByID(ctx context.Context, id uint) (*content.Comment, error)
    ListByArticle(ctx context.Context, articleID uint, page, size int) ([]content.Comment, int64, error)
    Update(ctx context.Context, comment *content.Comment) error
    Delete(ctx context.Context, id uint) error
}

type commentRepository struct {
    db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) CommentRepository {
    return &commentRepository{db: db}
}

func (r *commentRepository) Create(ctx context.Context, comment *content.Comment) error {
    return r.db.WithContext(ctx).Create(comment).Error
}

// ... 其他方法实现
```

#### 步骤3：创建DTO

```go
// internal/interface/admin/dto/content/comment.go
package content

// CreateCommentReq 创建评论请求
type CreateCommentReq struct {
    ArticleID uint   `json:"article_id" binding:"required"`
    Content   string `json:"content" binding:"required,max=500"`
}

// CommentResp 评论响应
type CommentResp struct {
    ID        uint   `json:"id"`
    ArticleID uint   `json:"article_id"`
    Content   string `json:"content"`
    Status    int8   `json:"status"`
    CreatedAt int64  `json:"created_at"`
}

// ListCommentReq 评论列表请求
type ListCommentReq struct {
    ArticleID uint `form:"article_id"`
    Page      int  `form:"page,default=1"`
    Size      int  `form:"size,default=20"`
}
```

#### 步骤4：创建Service

```go
// internal/service/content/comment.go
package content

import (
    "context"
    "server/internal/domain/entity/content"
    contentRepo "server/internal/repository/content"
)

type CommentService interface {
    Create(ctx context.Context, articleID uint, content string) (*content.Comment, error)
    ListByArticle(ctx context.Context, articleID uint, page, size int) ([]content.Comment, int64, error)
    Delete(ctx context.Context, id uint) error
}

type commentService struct {
    repo contentRepo.CommentRepository
}

func NewCommentService(repo contentRepo.CommentRepository) CommentService {
    return &commentService{repo: repo}
}

func (s *commentService) Create(ctx context.Context, articleID uint, contentStr string) (*content.Comment, error) {
    comment := &content.Comment{
        ArticleID: articleID,
        Content:   contentStr,
        Status:    1,
    }
    if err := s.repo.Create(ctx, comment); err != nil {
        return nil, err
    }
    return comment, nil
}

// ... 其他方法实现
```

#### 步骤5：创建Handler

```go
// internal/interface/admin/http/handler/v1/content/comment_handler.go
package content

import (
    "net/http"
    "server/internal/interface/admin/dto/content"
    "server/internal/pkg/errorx"
    "server/internal/pkg/response"
    "server/internal/service/content"

    "github.com/gin-gonic/gin"
)

type CommentHandler struct {
    service content.CommentService
}

func NewCommentHandler(service content.CommentService) *CommentHandler {
    return &CommentHandler{service: service}
}

func (h *CommentHandler) Create(c *gin.Context) {
    var req content.CreateCommentReq
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, errorx.CodeInvalidParams)
        return
    }

    comment, err := h.service.Create(c.Request.Context(), req.ArticleID, req.Content)
    if err != nil {
        response.Error(c, errorx.CodeInternalError)
        return
    }

    response.Success(c, comment)
}

func (h *CommentHandler) List(c *gin.Context) {
    var req content.ListCommentReq
    if err := c.ShouldBindQuery(&req); err != nil {
        response.Error(c, errorx.CodeInvalidParams)
        return
    }

    comments, total, err := h.service.ListByArticle(c.Request.Context(), req.ArticleID, req.Page, req.Size)
    if err != nil {
        response.Error(c, errorx.CodeInternalError)
        return
    }

    response.Success(c, gin.H{
        "list":  comments,
        "total": total,
    })
}
```

#### 步骤6：注册路由

```go
// internal/interface/admin/http/router/v1/content.go
// 在Register方法中添加：

func (r *ContentRouter) Register(group *gin.RouterGroup) {
    // ... 现有路由
    
    // 评论管理
    commentHandler := contentHandler.NewCommentHandler(r.commentService)
    commentGroup := group.Group("/comments")
    {
        commentGroup.GET("", commentHandler.List)
        commentGroup.POST("", commentHandler.Create)
        commentGroup.DELETE("/:id", commentHandler.Delete)
    }
}
```

#### 步骤7：更新Wire注入

```go
// internal/app/wire.go
// 在ProviderSet中添加：
// contentRepo.NewCommentRepository,
// contentService.NewCommentService,
```

---

## 五、关键规范

### 5.1 错误处理规范

- 使用 `internal/pkg/errorx` 中定义的错误码
- Handler层统一使用 `response.Error()` 或 `response.Success()` 返回
- Service层返回具体的业务错误，不处理HTTP响应

### 5.2 事务处理规范

```go
// 在Repository层处理事务
func (r *repository) CreateWithItems(ctx context.Context, data *Entity, items []Item) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := tx.Create(data).Error; err != nil {
            return err
        }
        if err := tx.Create(&items).Error; err != nil {
            return err
        }
        return nil
    })
}
```

### 5.3 日志规范

- 使用中间件自动记录操作日志
- 敏感字段（password、token等）自动脱敏
- 错误日志自动记录堆栈信息

---

## 六、相关文档

- [状态码规范](./status-codes.md)
- [API管理指南](./api-management.md)
- [缓存模块详解](./server-module-cache.md)
- [验证码模块详解](./server-module-captcha.md)
- [任务系统详解](./server-module-task.md)
- [字典模块详解](./server-module-dict.md)
- [内容管理模块详解](./server-module-content.md)
- [存储模块详解](./server-module-storage.md)
- [日志模块详解](./server-module-log.md)
- [数据迁移详解](./server-module-migration.md)
