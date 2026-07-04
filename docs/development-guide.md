# NetyAdmin 二次开发指南

本文档通过完整的**评论管理**模块示例，手把手教你在 NetyAdmin 基座上新增一个业务模块。覆盖从 Entity → Repository → DTO → Service → Handler → Router → Wire 注入的全流程。

> **前置阅读**：[Server 架构设计与目录结构](server-architecture.md) — 了解 BFF 多端隔离架构和分层设计理念。

---

## 一、开发前必读规范

### 1.1 分层调用链

```
Router → Handler → Service → Repository → Entity
```

| 层级 | 职责 | 红线 |
|------|------|------|
| **Handler** | 参数绑定 + 调 Service + 统一响应 | 禁止 import `domain/entity`；禁止调用 cacheMgr/repository |
| **Service** | 业务规则实现 + 多仓储聚合 | 禁止出现 `*gin.Context`；接收 DTO 不接收 entity；多步操作必须用 TM |
| **Repository** | CRUD + 查询拼装 | 禁止自管事务（`.Transaction()`）；通过 `getDB(ctx)` 统一取 DB |
| **Entity** | GORM 模型定义 | 纯数据结构，不含业务逻辑 |

### 1.2 核心红线速查

| 规则 | 说明 |
|------|------|
| **Repository 不自管事务** | 多步原子操作必须用 `TransactionManager` |
| **所有 repo 调用传 txCtx** | 事务内的 repo 用 Begin 返回的 `txCtx`，不是原始 `ctx` |
| **缓存失效在 Commit 之后** | 缓存清理 / 事件发布必须在 `tm.Commit()` 成功之后 |
| **Redis 操作在事务前** | `clearLoginLockCache` 等 Redis 操作不进 DB 事务 |
| **fail-closed** | 敏感操作（删除/禁用/改密）失败直接返回错误，不尝试补偿 |
| **DTO 专端专用** | Admin 端和 Client 端 DTO 独立存放，禁止跨端 import |
| **Handler 不 import entity** | 只接收 DTO，entity 构造下沉到 Service 层 |

### 1.3 统一响应格式

所有接口返回 HTTP 200，通过 `code` 字段区分业务结果：

```json
{
  "code": "100000",
  "msg": "",
  "data": {},
  "request_id": "xxx"
}
```

- 成功：`response.Success(c, data)` 或 `response.SuccessWithPage(c, current, size, total, list)`
- 业务错误：`response.Fail(c, err)` — Service 返回 `errorx.New(code)`
- 参数错误：`response.FailWithCode(c, errorx.CodeInvalidParams)`

---

## 二、完整示例：评论管理模块

### 步骤1：定义实体 Entity

```go
// internal/domain/entity/content/comment.go
package content

import (
	"gorm.io/plugin/soft_delete"
)

// Comment 评论实体
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

### 步骤2：创建仓储 Repository

**Repository 核心规范**：
- 只做 CRUD，**不自管事务**（禁止 `.Transaction(func(tx){...})`）
- 通过 `getDB(ctx)` 统一获取 `*gorm.DB`，不直接使用 `r.db`
- `getDB` 会自动判断 ctx 中是否有事务句柄：有则落入事务，无则走连接池

```go
// internal/repository/content/comment.go
package content

import (
	"context"

	"server/internal/domain/entity/content"
	"server/internal/pkg/database"
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

// getDB 通过 context 自动区分是否在事务中
func (r *commentRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *commentRepository) Create(ctx context.Context, comment *content.Comment) error {
	return r.getDB(ctx).Create(comment).Error
}

func (r *commentRepository) GetByID(ctx context.Context, id uint) (*content.Comment, error) {
	var comment content.Comment
	err := r.getDB(ctx).First(&comment, id).Error
	return &comment, err
}

func (r *commentRepository) ListByArticle(ctx context.Context, articleID uint, page, size int) ([]content.Comment, int64, error) {
	var comments []content.Comment
	var total int64
	db := r.getDB(ctx).Model(&content.Comment{}).Where("article_id = ?", articleID)
	db.Count(&total)
	err := db.Offset((page - 1) * size).Limit(size).Find(&comments).Error
	return comments, total, err
}

func (r *commentRepository) Update(ctx context.Context, comment *content.Comment) error {
	return r.getDB(ctx).Save(comment).Error
}

func (r *commentRepository) Delete(ctx context.Context, id uint) error {
	return r.getDB(ctx).Delete(&content.Comment{}, id).Error
}
```

> **Tip**：所有 Repository 方法内部都调 `r.getDB(ctx)`，绝不直接 `r.db.WithContext(ctx)`。这是为了支持事务传播：当 ctx 中包含事务句柄时，`getDB` 会自动落入事务。

### 步骤3：创建 DTO

**DTO 核心规范**：
- DTO 只含业务字段，不含 `ID`/`CreatedAt`/`UpdatedAt`/`DeletedAt` 等持久化字段
- Update DTO 的 ID 由 URL 参数传入，不放在 body 中
- Admin 端 DTO 放在 `interface/admin/dto/`，Client 端 DTO 放在 `interface/client/dto/`，**禁止跨端 import**

```go
// internal/interface/admin/dto/content/comment.go
package content

// CreateCommentReq 创建评论请求（不含持久化字段）
type CreateCommentReq struct {
	ArticleID uint   `json:"article_id" binding:"required"`
	Content   string `json:"content" binding:"required,max=500"`
}

// UpdateCommentReq 更新评论请求（ID 由 URL 传入，不放在 body）
type UpdateCommentReq struct {
	Content string `json:"content" binding:"max=500"`
	Status  int8   `json:"status" binding:"oneof=1 2"`
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

### 步骤4：创建 Service

**Service 层核心规范**：
- Service 接口接收 DTO，不接收 entity（entity 构造下沉到 Service 内部）
- Service 禁止出现 `*gin.Context`，只传 `context.Context` 和基础类型
- 多步原子操作必须使用 TransactionManager（TM），Repository 不自管事务

```go
// internal/service/content/comment.go
package content

import (
	"context"
	"log/slog"

	"server/internal/domain/entity/content"
	"server/internal/pkg/database"
	"server/internal/pkg/errorx"
	contentRepo "server/internal/repository/content"
	contentDto "server/internal/interface/admin/dto/content"
)

type CommentService interface {
	// Service 接口接收 DTO，不接收 entity
	Create(ctx context.Context, req *contentDto.CreateCommentReq) error
	Update(ctx context.Context, id uint, req *contentDto.UpdateCommentReq) error
	// 多步操作（如删除评论 + 递减评论数）使用 TM
	Delete(ctx context.Context, id uint) error
}

type commentService struct {
	repo contentRepo.CommentRepository
	tm   *database.TransactionManager
}

func NewCommentService(repo contentRepo.CommentRepository, tm *database.TransactionManager) CommentService {
	return &commentService{repo: repo, tm: tm}
}

// Create 接收 DTO，内部构造 entity
func (s *commentService) Create(ctx context.Context, req *contentDto.CreateCommentReq) error {
	comment := &content.Comment{
		ArticleID: req.ArticleID,
		Content:   req.Content,
		Status:    1,
	}
	return s.repo.Create(ctx, comment)
}

// Update 使用 GetByID + patch + Save 模式，避免 Save 全字段更新覆盖零值
func (s *commentService) Update(ctx context.Context, id uint, req *contentDto.UpdateCommentReq) error {
	old, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "评论不存在")
	}

	if req.Content != "" {
		old.Content = req.Content
	}
	if req.Status != 0 {
		old.Status = req.Status
	}

	return s.repo.Update(ctx, old)
}

// Delete + 递减评论数：TM 单事务原子完成
func (s *commentService) Delete(ctx context.Context, id uint) error {
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.repo.Delete(txCtx, id); err != nil {
		slog.Error("delete comment failed", "id", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "删除评论失败")
	}
	// 多步操作：递减文章评论数（必须用 txCtx，不用原始 ctx）
	if err := s.repo.DecrementCount(txCtx, id); err != nil {
		slog.Error("decrement comment count failed", "id", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "递减评论数失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("commit failed", "id", id, "err", err)
		return errorx.New(errorx.CodeInternalError, "事务提交失败")
	}
	return nil
}
```

### 步骤5：创建 Handler

**Handler 核心规范**：
- 只做协议转换：参数绑定 → 校验 → 调 Service → 统一响应，不含业务逻辑
- 禁止 import `domain/entity/` 包，只接收 DTO
- 禁止直接调用 cacheMgr/repository（应通过 Service 层完成）

```go
// internal/interface/admin/http/handler/v1/content/comment_handler.go
package content

import (
	"strconv"

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
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	// Service 接收 DTO
	if err := h.service.Create(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *CommentHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	var req content.UpdateCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	if err := h.service.Update(c.Request.Context(), uint(id), &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *CommentHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	if err := h.service.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
```

### 步骤6：注册路由

```go
// internal/interface/admin/http/router/v1/content.go
func (r *ContentRouter) Register(group *gin.RouterGroup) {
	// ... 现有路由

	// 评论管理
	commentHandler := contentHandler.NewCommentHandler(r.commentService)
	commentGroup := group.Group("/comments")
	{
		commentGroup.GET("", commentHandler.List)
		commentGroup.POST("", commentHandler.Create)
		commentGroup.PUT("/:id", commentHandler.Update)
		commentGroup.DELETE("/:id", commentHandler.Delete)
	}
}
```

### 步骤7：更新 Wire 注入

```go
// internal/app/wire.go
// 在 ProviderSet 中添加：
// contentRepo.NewCommentRepository,
// contentService.NewCommentService,
```

---

## 三、TransactionManager 事务指南

### 3.1 TM 架构

```
TransactionManager（无状态单例，DI 复用）
  Begin(ctx)     → (txCtx, tx)  开启事务，注入 context
  Commit(tx)     → 提交
  Rollback(tx)   → 回滚
  WithTransaction(ctx, fn) → 闭包事务（推荐）
    自动处理 panic/error 路径的 Rollback
                    │
                    ▼
Repository 通过 getDB(ctx) 统一取 *gorm.DB
  - ctx 中有事务 → 返回 tx.DB（落入事务）
  - ctx 中无事务 → 返回连接池（正常 CRUD）
```

### 3.2 TM 标准范式

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

### 3.3 DeleteBatch fail-closed 范式

```go
func (s *xxxService) DeleteBatch(ctx context.Context, ids []string) error {
	var skipped []string
	for _, id := range ids {
		// 业务规则拒绝：不存在的 id 跳过，不阻断
		if _, err := s.repo.GetByID(ctx, id); err != nil {
			skipped = append(skipped, fmt.Sprintf("id %s：不存在", id))
			continue
		}
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
		return errorx.New(errorx.CodeForbidden, fmt.Sprintf("部分被跳过：%s", strings.Join(skipped, "; ")))
	}
	return nil
}
```

### 3.4 TM 注入方式

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

---

## 四、DTO/Entity 隔离规范

### 4.1 为什么需要隔离？

- Handler 只做协议转换，不应知道 entity 结构
- Service 通过 DTO 明确入参边界，不受 entity 字段变化影响
- Admin/Client 两端入参完全不同，统一 DTO 会导致混乱

### 4.2 规范细则

**DTO 定义**：
- 只含业务字段，不含 `ID`/`CreatedAt`/`UpdatedAt`/`DeletedAt`/`Password` 等持久化字段
- `CreateXxxReq` 包含创建所需的所有字段（不含 ID）
- `UpdateXxxReq` 只含可修改的业务字段（ID 由 URL `:id` 传入）
- 两端 DTO **禁止跨端 import**

**Service 接口签名**：
```go
Create(ctx, req *dto.CreateXxxReq) error
Update(ctx, id uint64, req *dto.UpdateXxxReq) error
```

**Handler**：
- 禁止 import `domain/entity/` 包
- Update 的 ID 从 `c.Param("id")` 解析

### 4.3 Update 实现：GetByID + patch + Save

GORM 的 `Save` 是全字段更新。如果 DTO 字段少于 entity，零值会覆盖数据库已有值（如 `CreatedAt`/`DeletedAt`）。解决方案是先 GetByID 取旧值，再 patch 业务字段：

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

### 4.4 BFF Service 端隔离

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
}
type userAdminService struct{ userBase }
func NewUserAdminService(base userBase) UserAdminService {
	return &userAdminService{userBase: base}
}

// Client 端 service：仅 import client/dto/v1
type UserClientService interface {
	Register(ctx context.Context, req *clientDto.UserRegisterReq) error
	DeleteAccount(ctx context.Context, userID string) error
}
type userClientService struct{ userBase }
func NewUserClientService(base userBase) UserClientService {
	return &userClientService{userBase: base}
}

// wire.go 注入
userBase := userService.NewUserBase(...)
s.userAdmin = userService.NewUserAdminService(userBase)
s.userClient = userService.NewUserClientService(userBase)
```

---

## 五、常见踩坑

| 问题 | 原因 | 解决 |
|------|------|------|
| **事务内 repo 不生效** | repo 方法用了原始 `ctx` 而非 `txCtx` | 所有事务内 repo 调用必须传 `txCtx` |
| **缓存失效但 DB 未改** | 缓存失效在 Commit 之前，Commit 失败了 | 缓存失效/事件发布必须在 `tm.Commit()` 之后 |
| **Save 后某些字段变零值** | DTO 字段少于 entity，零值覆盖了 DB 已有值 | 用 GetByID + patch + Save 模式 |
| **构建报 undefined 但代码看起来正确** | Go 构建缓存陈旧 | `go clean -cache` 后 `go build ./...` |
| **DeleteBatch 部分失败但前面的已提交** | 这是 fail-closed 的预期行为 | 不存在的 id 跳过，事务失败立即返回 |
| **PowerShell 执行 Go 命令失败** | 用了 `&&` 分隔命令 | PowerShell 用 `;` 分隔：`cd server; go build ./...` |

---

## 六、相关文档

- [Server 架构设计与目录结构](server-architecture.md)
- [状态码规范](status-codes.md)
- [API 管理指南](api-management.md)
- [RULES.md](../RULES.md) — 红线规则（**本地保留，不推 GitHub**）