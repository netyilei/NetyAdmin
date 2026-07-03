package user

// user_admin.go - 后台管理相关方法：列表、搜索、创建、更新、状态变更、删除、水印

import (
	"context"
	"fmt"
	"strings"

	userEntity "NetyAdmin/internal/domain/entity/user"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/password"
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
		hashedPassword, err := password.Hash(user.Password)
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
		hashedPassword, err := password.Hash(user.Password)
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
	// 删除用户：失效所有 token + 递增版本号（防止被删用户继续访问）
	if err := s.invalidateUserTokens(ctx, id); err != nil {
		return errorx.New(errorx.CodeInternalError, "令牌失效失败")
	}
	return s.repo.Delete(ctx, id)
}

func (s *userService) DeleteBatch(ctx context.Context, ids []string) error {
	// 批量删除：逐个失效 token（版本号机制要求逐条 UPDATE）。
	// 与 admin_manage.go DeleteBatch 对齐：失败记录到 errs，最终聚合返回，避免静默吞错。
	var errs []string
	for _, id := range ids {
		if err := s.invalidateUserTokens(ctx, id); err != nil {
			errs = append(errs, fmt.Sprintf("用户 %s：令牌失效失败", id))
		}
	}
	if err := s.repo.DeleteBatch(ctx, ids); err != nil {
		return err
	}
	if len(errs) > 0 {
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("部分用户令牌失效失败：%s", strings.Join(errs, "; ")))
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
