package user

// user_admin.go - 后台管理用户服务：UserAdminService 接口、userAdminService 实现，
// 以及 admin 端方法（列表、搜索、创建、更新、状态变更、解锁、删除、批量删除）。
//
// 仅 import admin/dto/user，不 import client/dto/v1，保证 BFF 端隔离（spec D4）。
// 共享横切逻辑（密码强度校验、登录锁清理）通过嵌入 userBase 复用。

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	userEntity "NetyAdmin/internal/domain/entity/user"
	userDto "NetyAdmin/internal/interface/admin/dto/user"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	passwordPkg "NetyAdmin/internal/pkg/password"
	"NetyAdmin/internal/pkg/utils"
	userRepo "NetyAdmin/internal/repository/user"
)

// UserAdminService 是 admin 端用户服务接口。
// 仅包含 admin handler 调用的方法，入参为 admin DTO（禁止 entity 入参，spec D4）。
type UserAdminService interface {
	List(ctx context.Context, req *userDto.UserQuery) ([]UserWithLock, int64, error)
	SearchForAutocomplete(ctx context.Context, keyword string, limit int) ([]userEntity.User, error)
	Create(ctx context.Context, req *userDto.CreateUserReq) error
	Update(ctx context.Context, id string, req *userDto.UpdateUserReq) error
	UpdateStatus(ctx context.Context, id string, status string) error
	UnlockUser(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	DeleteBatch(ctx context.Context, ids []string) error
}

type userAdminService struct {
	userBase
}

// NewUserAdminService 基于 userBase 构造 admin 端用户服务。
func NewUserAdminService(base userBase) UserAdminService {
	return &userAdminService{userBase: base}
}

// UserWithLock 是 admin List 场景的返回 VO，嵌入 User 实体并附加 Locked 字段（登录锁定状态）。
// 由 service 层完成 cacheSlow 查询填充，admin handler 不再直接操作 cacheSlow（spec B10）。
// entity 中的 Password / DeletedAt / TokenVersion 均带 json:"-"，序列化安全。
type UserWithLock struct {
	userEntity.User
	Locked bool `json:"locked"`
}

func (s *userAdminService) List(ctx context.Context, req *userDto.UserQuery) ([]UserWithLock, int64, error) {
	// service 层接收 admin DTO，内部构造 repository query（spec B10：service 不应依赖 handler 构造的 repo 类型）
	repoQuery := &userRepo.UserRepoQuery{
		Current:  req.Current,
		Size:     req.Size,
		Username: req.Username,
		Nickname: req.Nickname,
		Gender:   req.Gender,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
	}
	users, total, err := s.repo.List(ctx, repoQuery)
	if err != nil {
		return nil, 0, err
	}

	// 在 service 层完成 locked 状态查询，避免 handler 直接操作 cacheSlow（spec B10）
	items := make([]UserWithLock, 0, len(users))
	for _, u := range users {
		locked := false
		var lockVal string
		lockKey := cache.KeyLoginLock(u.ID)
		if err := s.cacheSlow.Get(ctx, lockKey, &lockVal); err == nil && lockVal != "" {
			locked = true
		}
		items = append(items, UserWithLock{User: u, Locked: locked})
	}
	return items, total, nil
}

func (s *userAdminService) SearchForAutocomplete(ctx context.Context, keyword string, limit int) ([]userEntity.User, error) {
	return s.repo.SearchForAutocomplete(ctx, keyword, limit)
}

// Create 创建用户。entity 构造下沉到 service 层（spec D4：禁止 entity 入参）。
func (s *userAdminService) Create(ctx context.Context, req *userDto.CreateUserReq) error {
	// 1. 检查唯一性
	exists, existsErr := s.repo.ExistsByUsername(ctx, req.Username)
	if existsErr != nil {
		slog.Warn("ExistsByUsername query failed (rely on DB unique constraint as fallback)", "username", req.Username, "error", existsErr)
	}
	if exists {
		return errorx.New(errorx.CodeUserAlreadyExists, "用户名已存在")
	}
	if req.Phone != "" {
		exists, existsErr = s.repo.ExistsByPhone(ctx, req.Phone)
		if existsErr != nil {
			slog.Warn("ExistsByPhone query failed", "phone", req.Phone, "error", existsErr)
		}
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
	}
	if req.Email != "" {
		exists, existsErr = s.repo.ExistsByEmail(ctx, req.Email)
		if existsErr != nil {
			slog.Warn("ExistsByEmail query failed", "email", req.Email, "error", existsErr)
		}
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
	}

	// 2. 密码加密
	password := req.Password
	if password != "" {
		if err := s.validatePasswordStrength(ctx, password); err != nil {
			return err
		}
		hashedPassword, err := passwordPkg.Hash(password)
		if err != nil {
			return errorx.New(errorx.CodeInternalError, "密码加密失败")
		}
		password = hashedPassword
	}

	// 3. 构造 entity 并设置 ID + 默认状态
	status := req.Status
	if status == "" {
		status = entity.StatusEnabled
	}
	user := &userEntity.User{
		ID:       utils.NewULID(),
		Username: req.Username,
		Password: password,
		Nickname: req.Nickname,
		Avatar:   req.Avatar,
		Gender:   req.Gender,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   status,
	}

	return s.repo.Create(ctx, user)
}

// Update 更新用户。先 GetByID 取旧值，再按 DTO 字段 patch，避免 Save 全字段更新覆盖零值（spec D4 BUG 修复）。
// 密码或状态置禁用属敏感变更，通过 TM 单事务原子完成「递增 token_version + 更新用户」（fail-closed）。
func (s *userAdminService) Update(ctx context.Context, id string, req *userDto.UpdateUserReq) error {
	oldUser, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeUserNotFound, "用户不存在")
		}
		slog.Error("repo.GetByID failed", "userID", id, "err", err)
		return fmt.Errorf("repo.GetByID: %w", err)
	}

	// 1. 检查唯一性（admin 不支持通过 Update 修改 username，仅校验 phone/email 唯一性）
	var (
		exists    bool
		existsErr error
	)
	if req.Phone != "" && req.Phone != oldUser.Phone {
		exists, existsErr = s.repo.ExistsByPhone(ctx, req.Phone, id)
		if existsErr != nil {
			slog.Warn("ExistsByPhone query failed", "phone", req.Phone, "error", existsErr)
		}
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
		oldUser.Phone = req.Phone
	}
	if req.Email != "" && req.Email != oldUser.Email {
		exists, existsErr = s.repo.ExistsByEmail(ctx, req.Email, id)
		if existsErr != nil {
			slog.Warn("ExistsByEmail query failed", "email", req.Email, "error", existsErr)
		}
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
		oldUser.Email = req.Email
	}

	// 2. 处理密码更新（敏感操作：后续单事务原子「递增 token_version + 更新用户」）
	passwordChanged := false
	if req.Password != "" {
		if err := s.validatePasswordStrength(ctx, req.Password); err != nil {
			return err
		}
		hashedPassword, err := passwordPkg.Hash(req.Password)
		if err != nil {
			return errorx.New(errorx.CodeInternalError, "密码加密失败")
		}
		oldUser.Password = hashedPassword
		passwordChanged = true
	}

	// 3. 更新其他字段
	if req.Nickname != "" {
		oldUser.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		oldUser.Avatar = req.Avatar
	}
	if req.Gender != "" {
		oldUser.Gender = req.Gender
	}
	// statusWillDisable 标记本次操作是否要将用户置为禁用。
	// 置为禁用属于敏感操作，需通过单事务原子「递增 token_version + 更新用户」完成（spec A3/A5）。
	statusWillDisable := false
	if req.Status != "" && req.Status != oldUser.Status {
		oldUser.Status = req.Status
		// 状态变更：
		// - 禁用：递增 token_version 失效所有旧 token + 清登录锁（清锁在事务前调用，Redis 不进事务）
		// - 启用：仅清登录锁（用户重新获得登录资格，不递增版本号）
		if req.Status == entity.StatusDisabled {
			statusWillDisable = true
		} else if req.Status == entity.StatusEnabled {
			s.clearLoginLockCache(ctx, id)
		}
	}

	// 密码或状态置禁用属敏感变更，统一通过 TM 单事务原子完成「递增 token_version + 更新用户」（fail-closed）。
	// 任一步失败整体回滚，避免「密码已改但版本号未递增」或「版本号递增但密码未改」的中间态（spec A3）。
	// clearLoginLockCache 在事务前调用（Redis 操作不进事务）。
	if passwordChanged || statusWillDisable {
		s.clearLoginLockCache(ctx, id)
		txCtx, tx := s.tm.Begin(ctx)
		if err := s.repo.IncrementTokenVersion(txCtx, id); err != nil {
			slog.Error("user update: increment token version failed", "userID", id, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "令牌失效失败")
		}
		if err := s.repo.Update(txCtx, oldUser); err != nil {
			slog.Error("user update: update user failed", "userID", id, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "用户更新失败")
		}
		if err := s.tm.Commit(tx); err != nil {
			slog.Error("user update: commit failed", "userID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "事务提交失败")
		}
		return nil
	}
	return s.repo.Update(ctx, oldUser)
}

func (s *userAdminService) UpdateStatus(ctx context.Context, id string, status string) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("repo.GetByID: %w", err)
	}

	// 状态未变更，直接返回
	if user.Status == status {
		return nil
	}

	user.Status = status

	// 状态变更处理（语义同 Update 中的状态分支）：
	// - 禁用：TM 单事务原子「递增 token_version + 更新用户」失效所有旧 token（fail-closed）
	// - 启用：仅清登录锁（不递增版本号）
	// clearLoginLockCache 在事务前调用（Redis 操作不进事务）。
	if status == entity.StatusDisabled {
		s.clearLoginLockCache(ctx, id)
		txCtx, tx := s.tm.Begin(ctx)
		if err := s.repo.IncrementTokenVersion(txCtx, id); err != nil {
			slog.Error("user update status: increment token version failed", "userID", id, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "令牌失效失败")
		}
		if err := s.repo.Update(txCtx, user); err != nil {
			slog.Error("user update status: update user failed", "userID", id, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "用户更新失败")
		}
		if err := s.tm.Commit(tx); err != nil {
			slog.Error("user update status: commit failed", "userID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "事务提交失败")
		}
		return nil
	} else if status == entity.StatusEnabled {
		s.clearLoginLockCache(ctx, id)
	}

	return s.repo.Update(ctx, user)
}

// UnlockUser 解除用户登录锁定状态。
// 仅清理 Redis 中的登录锁/重试计数缓存，不涉及 DB 写操作。
// 抽取到 service 层，避免 handler 直接操作 cacheSlow（spec B10）。
func (s *userAdminService) UnlockUser(ctx context.Context, id string) error {
	s.clearLoginLockCache(ctx, id)
	return nil
}

func (s *userAdminService) Delete(ctx context.Context, id string) error {
	// TM 单事务原子完成「递增 token_version + 软删除」（fail-closed），与 DeleteBatch 语义一致。
	// 任一步失败整体回滚，避免「版本号已变但主数据未删」或「主数据已删但版本号未变」的中间态。
	// clearLoginLockCache 在事务前调用：清理无关缓存，不参与事务（即使失败也不影响删除主流程）。
	s.clearLoginLockCache(ctx, id)
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.repo.IncrementTokenVersion(txCtx, id); err != nil {
		slog.Error("user delete: increment token version failed", "userID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("用户 %s 删除失败", id))
	}
	if err := s.repo.Delete(txCtx, id); err != nil {
		slog.Error("user delete: delete user failed", "userID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("用户 %s 删除失败", id))
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("user delete: commit failed", "userID", id, "err", err)
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("用户 %s 删除失败", id))
	}
	return nil
}

func (s *userAdminService) DeleteBatch(ctx context.Context, ids []string) error {
	// 逐条事务 fail-closed：每个 id 走 TM 单事务（IncrementTokenVersion + Delete），
	// 任一 id 事务失败立即返回错误，已提交的 id 保持删除状态，未处理的 id 不受影响。
	// 不存在的 id 走 continue 跳过并记录到 skipped，不阻断整个批量（与 admin/role 模式对齐）。
	//
	// 设计权衡（vs 旧 fail-open 实现）：
	//   - 安全优先：避免 IncrementTokenVersion 失败但 Delete 成功导致"已删但版本号未变"的中间态
	//   - 一致性优先：TM 单事务原子保证「要么两步都成功，要么都回滚」
	//   - 已提交 id 不回滚：事务一旦 commit 即不可撤销，符合「逐条」语义
	//
	// clearLoginLockCache 在每条事务前调用（仅对存在的用户），清理无关缓存（Redis 操作不进事务），
	// 避免被删用户的登录锁/重试计数在 Redis 中残留至 TTL 过期。
	//
	// 性能：N 个 id = N 次事务，相比旧实现 1 次 DeleteBatch + N 次 UPDATE 略慢，
	// 但 DeleteBatch 是低频管理操作，可接受。
	var skipped []string
	for _, id := range ids {
		if _, err := s.repo.GetByID(ctx, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				skipped = append(skipped, fmt.Sprintf("用户 %s：不存在", id))
				continue
			}
			slog.Error("repo.GetByID failed", "userID", id, "err", err)
			return fmt.Errorf("repo.GetByID: %w", err)
		}
		s.clearLoginLockCache(ctx, id)
		txCtx, tx := s.tm.Begin(ctx)
		if err := s.repo.IncrementTokenVersion(txCtx, id); err != nil {
			slog.Error("user delete batch: increment token version failed", "userID", id, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("用户 %s 删除失败", id))
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			slog.Error("user delete batch: delete user failed", "userID", id, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("用户 %s 删除失败", id))
		}
		if err := s.tm.Commit(tx); err != nil {
			slog.Error("user delete batch: commit failed", "userID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("用户 %s 删除失败", id))
		}
	}
	if len(skipped) > 0 {
		return errorx.New(errorx.CodeForbidden, fmt.Sprintf("部分用户被跳过：%s", strings.Join(skipped, "; ")))
	}
	return nil
}
