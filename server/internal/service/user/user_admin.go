package user

// user_admin.go - 后台管理相关方法：列表、搜索、创建、更新、状态变更、删除、水印

import (
	"context"

	userEntity "NetyAdmin/internal/domain/entity/user"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/password"
	"NetyAdmin/internal/pkg/utils"
	userRepo "NetyAdmin/internal/repository/user"
)

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
	if user.Username != "" && user.Username != oldUser.Username {
		exists, _ := s.repo.ExistsByUsername(ctx, user.Username)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "用户名已存在")
		}
		oldUser.Username = user.Username
	}
	if user.Phone != "" && user.Phone != oldUser.Phone {
		exists, _ := s.repo.ExistsByPhone(ctx, user.Phone, user.ID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
		oldUser.Phone = user.Phone
	}
	if user.Email != "" && user.Email != oldUser.Email {
		exists, _ := s.repo.ExistsByEmail(ctx, user.Email, user.ID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
		oldUser.Email = user.Email
	}

	// 2. 处理密码更新
	if user.Password != "" {
		if err := s.validatePasswordStrength(ctx, user.Password); err != nil {
			return err
		}
		hashedPassword, err := password.Hash(user.Password)
		if err != nil {
			return errorx.New(errorx.CodeInternalError, "密码加密失败")
		}
		oldUser.Password = hashedPassword
		// 强制清理 Token
		_ = s.tokenStore.DeleteAll(ctx, user.ID)
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
		// 状态变更时同步处理 Token 与登录锁定缓存：
		// - 禁用：立即拉黑所有 Token，防止被冻结用户继续访问
		// - 启用：清理历史登录失败计数与锁定状态，避免恢复后仍被拦截
		if user.Status == entity.StatusDisabled {
			_ = s.tokenStore.DeleteAll(ctx, user.ID)
			_ = s.cacheMgr.Delete(ctx, cache.KeyLoginLock(user.ID))
			_ = s.cacheMgr.Delete(ctx, cache.KeyLoginRetryCount(user.ID))
		} else if user.Status == entity.StatusEnabled {
			_ = s.cacheMgr.Delete(ctx, cache.KeyLoginLock(user.ID))
			_ = s.cacheMgr.Delete(ctx, cache.KeyLoginRetryCount(user.ID))
		}
	}

	return s.repo.Update(ctx, oldUser)
}

func (s *userService) UpdateStatus(ctx context.Context, id string, status string) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 状态未变更，直接返回，避免重复清理 Token 与缓存
	if user.Status == status {
		return nil
	}

	user.Status = status

	// 状态变更时同步处理 Token 与登录锁定缓存：
	// - 禁用：立即拉黑所有 Token，防止被冻结用户继续访问
	// - 启用：清理历史登录失败计数与锁定状态，避免恢复后仍被拦截
	if status == entity.StatusDisabled {
		_ = s.tokenStore.DeleteAll(ctx, id)
		_ = s.cacheMgr.Delete(ctx, cache.KeyLoginLock(id))
		_ = s.cacheMgr.Delete(ctx, cache.KeyLoginRetryCount(id))
	} else if status == entity.StatusEnabled {
		_ = s.cacheMgr.Delete(ctx, cache.KeyLoginLock(id))
		_ = s.cacheMgr.Delete(ctx, cache.KeyLoginRetryCount(id))
	}

	return s.repo.Update(ctx, user)
}

func (s *userService) Delete(ctx context.Context, id string) error {
	// 删除用户后，清除其所有 token，防止被删除用户继续访问
	_ = s.tokenStore.DeleteAll(ctx, id)
	_ = s.cacheMgr.Delete(ctx, cache.KeyLoginLock(id))
	_ = s.cacheMgr.Delete(ctx, cache.KeyLoginRetryCount(id))
	return s.repo.Delete(ctx, id)
}

func (s *userService) DeleteBatch(ctx context.Context, ids []string) error {
	for _, id := range ids {
		// 批量删除后，清除对应用户所有 token
		_ = s.tokenStore.DeleteAll(ctx, id)
		_ = s.cacheMgr.Delete(ctx, cache.KeyLoginLock(id))
		_ = s.cacheMgr.Delete(ctx, cache.KeyLoginRetryCount(id))
	}
	return s.repo.DeleteBatch(ctx, ids)
}

func (s *userService) UpdateLastReadID(ctx context.Context, userID string, lastReadID uint64) error {
	return s.repo.UpdateFields(ctx, userID, map[string]interface{}{
		"last_read_announcement_id": lastReadID,
	})
}

func (s *userService) DeleteAccount(ctx context.Context, userID string) error {
	_ = s.tokenStore.DeleteAll(ctx, userID)
	_ = s.cacheMgr.Delete(ctx, cache.KeyLoginLock(userID))
	_ = s.cacheMgr.Delete(ctx, cache.KeyLoginRetryCount(userID))
	return s.repo.Delete(ctx, userID)
}
