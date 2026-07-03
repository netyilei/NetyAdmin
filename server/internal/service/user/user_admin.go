package user

// user_admin.go - 后台管理相关方法：列表、搜索、创建、更新、状态变更、删除、水印

import (
	"context"
	"fmt"

	userEntity "NetyAdmin/internal/domain/entity/user"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	passwordPkg "NetyAdmin/internal/pkg/password"
	"NetyAdmin/internal/pkg/utils"
	userRepo "NetyAdmin/internal/repository/user"
)

// clearLoginLockCache 清理用户登录锁定/重试计数缓存。
// 提取此 helper 消除 user_admin.go 中 5 处复制粘贴（RULES.md §0.1 / 重构清单 B-AUTH-4）。
func (s *userService) clearLoginLockCache(ctx context.Context, userID string) {
	_ = s.cacheMgr.Delete(ctx, cache.KeyLoginLock(userID))
	_ = s.cacheMgr.Delete(ctx, cache.KeyLoginRetryCount(userID))
}

func (s *userService) List(ctx context.Context, current, size int, query *userRepo.UserRepoQuery) ([]userEntity.User, int64, error) {
	query.Current = current
	query.Size = size
	return s.repo.List(ctx, query)
}

func (s *userService) SearchForAutocomplete(ctx context.Context, keyword string, limit int) ([]userEntity.User, error) {
	return s.repo.SearchForAutocomplete(ctx, keyword, limit)
}

func (s *userService) Create(ctx context.Context, user *userEntity.User) error {
	// 1. 检查唯一性
	exists, _ := s.repo.ExistsByUsername(ctx, user.Username)
	if exists {
		return errorx.New(errorx.CodeUserAlreadyExists, "用户名已存在")
	}
	if user.Phone != "" {
		exists, _ = s.repo.ExistsByPhone(ctx, user.Phone)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
	}
	if user.Email != "" {
		exists, _ = s.repo.ExistsByEmail(ctx, user.Email)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
	}

	// 2. 密码加密
	if user.Password != "" {
		if err := s.validatePasswordStrength(ctx, user.Password); err != nil {
			return err
		}
		hashedPassword, err := passwordPkg.Hash(user.Password)
		if err != nil {
			return errorx.New(errorx.CodeInternalError, "密码加密失败")
		}
		user.Password = hashedPassword
	}

	// 3. 设置 ID 和默认状态
	if user.ID == "" {
		user.ID = utils.NewULID()
	}
	if user.Status == "" {
		user.Status = entity.StatusEnabled
	}

	return s.repo.Create(ctx, user)
}

func (s *userService) Update(ctx context.Context, user *userEntity.User) error {
	oldUser, err := s.repo.GetByID(ctx, user.ID)
	if err != nil {
		return errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	// 1. 检查唯一性
	var exists bool
	if user.Username != "" && user.Username != oldUser.Username {
		exists, _ = s.repo.ExistsByUsername(ctx, user.Username)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "用户名已存在")
		}
		oldUser.Username = user.Username
	}
	if user.Phone != "" && user.Phone != oldUser.Phone {
		exists, _ = s.repo.ExistsByPhone(ctx, user.Phone, user.ID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
		oldUser.Phone = user.Phone
	}
	if user.Email != "" && user.Email != oldUser.Email {
		exists, _ = s.repo.ExistsByEmail(ctx, user.Email, user.ID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
		oldUser.Email = user.Email
	}

	// 2. 处理密码更新（敏感操作：失效所有旧 token + 递增版本号）
	if user.Password != "" {
		if err := s.validatePasswordStrength(ctx, user.Password); err != nil {
			return err
		}
		hashedPassword, err := passwordPkg.Hash(user.Password)
		if err != nil {
			return errorx.New(errorx.CodeInternalError, "密码加密失败")
		}
		oldUser.Password = hashedPassword
		// 失效 token：tokenStore.DeleteAll + IncrementTokenVersion（fail-closed）
		if err := s.invalidateUserTokens(ctx, user.ID); err != nil {
			return errorx.New(errorx.CodeInternalError, "令牌失效失败")
		}
	}

	// 3. 更新其他字段
	if user.Nickname != "" {
		oldUser.Nickname = user.Nickname
	}
	if user.Avatar != "" {
		oldUser.Avatar = user.Avatar
	}
	if user.Gender != "" {
		oldUser.Gender = user.Gender
	}
	if user.Status != "" && user.Status != oldUser.Status {
		oldUser.Status = user.Status
		// 状态变更：
		// - 禁用：失效所有 token（含版本号递增）+ 清登录锁
		// - 启用：仅清登录锁（用户重新获得登录资格，不递增版本号）
		if user.Status == entity.StatusDisabled {
			if err := s.invalidateUserTokens(ctx, user.ID); err != nil {
				return errorx.New(errorx.CodeInternalError, "令牌失效失败")
			}
		} else if user.Status == entity.StatusEnabled {
			s.clearLoginLockCache(ctx, user.ID)
		}
	}

	return s.repo.Update(ctx, oldUser)
}

func (s *userService) UpdateStatus(ctx context.Context, id string, status string) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 状态未变更，直接返回
	if user.Status == status {
		return nil
	}

	user.Status = status

	// 状态变更处理（语义同 Update 中的状态分支）
	if status == entity.StatusDisabled {
		if err := s.invalidateUserTokens(ctx, id); err != nil {
			return errorx.New(errorx.CodeInternalError, "令牌失效失败")
		}
	} else if status == entity.StatusEnabled {
		s.clearLoginLockCache(ctx, id)
	}

	return s.repo.Update(ctx, user)
}

func (s *userService) Delete(ctx context.Context, id string) error {
	// 单事务原子完成「递增 token_version + 软删除」，与 DeleteBatch 语义一致。
	// 不再用旧的 invalidateUserTokens + repo.Delete 两步分离（避免 Inc 成功+Delete 失败的中间态）。
	// clearLoginLockCache 在事务前调用：清理无关缓存，不参与事务（即使失败也不影响删除主流程）。
	s.clearLoginLockCache(ctx, id)
	if err := s.repo.DeleteWithTokenInvalidation(ctx, id); err != nil {
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("用户 %s 删除失败（事务回滚）：%s", id, err.Error()))
	}
	return nil
}

func (s *userService) DeleteBatch(ctx context.Context, ids []string) error {
	// 逐条事务 fail-closed：每个 id 走单事务（IncrementTokenVersion + Delete），
	// 任一 id 事务失败立即返回错误，已提交的 id 保持删除状态，未处理的 id 不受影响。
	//
	// 设计权衡（vs 旧 fail-open 实现）：
	//   - 安全优先：避免 IncrementTokenVersion 失败但 DeleteBatch 成功导致"已删但版本号未变"的中间态
	//   - 一致性优先：单事务原子保证「要么两步都成功，要么都回滚」
	//   - 已提交 id 不回滚：事务一旦 commit 即不可撤销，符合「逐条」语义
	//
	// clearLoginLockCache 在每条事务前调用（补回旧 invalidateUserTokens 的清理逻辑，
	// 避免被删用户的登录锁/重试计数在 Redis 中残留至 TTL 过期）。
	//
	// 性能：N 个 id = N 次事务，相比旧实现 1 次 DeleteBatch + N 次 UPDATE 略慢，
	// 但 DeleteBatch 是低频管理操作，可接受。
	for _, id := range ids {
		s.clearLoginLockCache(ctx, id)
		if err := s.repo.DeleteWithTokenInvalidation(ctx, id); err != nil {
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("用户 %s 删除失败（事务回滚）：%s", id, err.Error()))
		}
	}
	return nil
}

func (s *userService) UpdateLastReadID(ctx context.Context, userID string, lastReadID uint64) error {
	return s.repo.UpdateFields(ctx, userID, map[string]interface{}{
		"last_read_announcement_id": lastReadID,
	})
}

func (s *userService) DeleteAccount(ctx context.Context, userID string) error {
	if err := s.invalidateUserTokens(ctx, userID); err != nil {
		return errorx.New(errorx.CodeInternalError, "令牌失效失败")
	}
	return s.repo.Delete(ctx, userID)
}
