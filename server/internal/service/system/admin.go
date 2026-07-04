// admin.go 管理员服务：接口定义、构造函数、工具函数及个人资料相关方法。
// 认证相关方法见 admin_auth.go，增删改查相关方法见 admin_manage.go。
package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"

	systemVO "NetyAdmin/internal/domain/vo/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/password"
	systemRepo "NetyAdmin/internal/repository/system"
	userService "NetyAdmin/internal/service/user"
)

type AdminService interface {
	Login(ctx context.Context, req *systemDto.LoginReq) (*systemVO.LoginVO, error)
	// Logout 退出登录：删除 access token hash，并将 refresh token 写入黑名单（TTL 为其剩余有效期），
	// 防止登出后 refresh token 仍可用于换取新 access token（P0-1 BUG 修复）。
	Logout(ctx context.Context, adminID uint, accessToken, refreshToken string) error
	RefreshToken(ctx context.Context, refreshToken string) (*systemVO.LoginVO, error)
	GetAdminInfo(ctx context.Context, adminID uint) (*systemVO.AdminInfoVO, error)
	GetProfile(ctx context.Context, adminID uint) (*systemVO.ProfileVO, error)
	UpdateProfile(ctx context.Context, adminID uint, req *systemDto.UpdateProfileReq) error
	ChangePassword(ctx context.Context, adminID uint, req *systemDto.ChangePasswordReq) error
	List(ctx context.Context, req *systemDto.AdminQuery) ([]*systemVO.AdminItemVO, int64, error)
	Create(ctx context.Context, req *systemDto.CreateAdminReq, operatorID uint, operatorIsSuper bool) (uint, error)
	Update(ctx context.Context, req *systemDto.UpdateAdminReq, operatorID uint, operatorIsSuper bool) error
	Delete(ctx context.Context, id uint, operatorID uint) error
	GetByID(ctx context.Context, id uint) (*systemEntity.Admin, error)
	GetByUsername(ctx context.Context, username string) (*systemEntity.Admin, error)
	ExistsByUsername(ctx context.Context, username string, excludeID ...uint) (bool, error)
	DeleteBatch(ctx context.Context, ids []uint, operatorID uint) error
}

type adminService struct {
	adminRepo  systemRepo.AdminRepository
	roleRepo   systemRepo.RoleRepository
	jwt        *jwt.JWT
	cacheMgr   cache.LazyCacheManager
	tokenStore userService.TokenStore
	tm         *database.TransactionManager
}

func NewAdminService(adminRepo systemRepo.AdminRepository, roleRepo systemRepo.RoleRepository, jwtInstance *jwt.JWT, cacheMgr cache.LazyCacheManager, tokenStore userService.TokenStore, tm *database.TransactionManager) AdminService {
	return &adminService{
		adminRepo:  adminRepo,
		roleRepo:   roleRepo,
		jwt:        jwtInstance,
		cacheMgr:   cacheMgr,
		tokenStore: tokenStore,
		tm:         tm,
	}
}

// validateAdminPasswordStrength 校验管理员密码强度。
// 委托给 pkg/password.ValidateStrength，使用管理员端默认配置（3 类字符）。
// 注：原始校验错误用 slog 记录，message 不含 err.Error() 内部细节，避免泄露校验实现。
func validateAdminPasswordStrength(pwd string) error {
	if err := password.ValidateStrength(pwd, password.DefaultAdminStrengthConfig); err != nil {
		slog.Warn("admin password strength validation failed", "err", err)
		return errorx.New(errorx.CodePasswordTooWeak, "密码强度不足")
	}
	return nil
}

// invalidateAdminAuthStateCache 失效管理员鉴权状态缓存（按 adminID 精准）。
// 用于 token_version 变更后保证下次鉴权重算，避免 30s TTL 窗口内旧 token 绕过版本号校验。
// 失败仅记录日志不阻断：DB 层 token_version 已是最终值，缓存最长 30s 后自然过期。
func (s *adminService) invalidateAdminAuthStateCache(ctx context.Context, adminID uint) {
	if s.cacheMgr == nil {
		return
	}
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminAuthByID(adminID)); err != nil {
		slog.Error("invalidate admin auth_state cache failed",
			"adminID", adminID, "err", err)
	}
}

// invalidateAdminInfoCache 全局失效 admin:info 缓存（列表页/详情页可能引用了已变更的 admin）。
// 用于 admin 增删改后保证列表/详情页重算。
// 失败仅记录日志不阻断：DB 已是最终状态，缓存最长 TTL 后自然过期。
func (s *adminService) invalidateAdminInfoCache(ctx context.Context) {
	if s.cacheMgr == nil {
		return
	}
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo); err != nil {
		slog.Error("invalidate admin info cache failed", "err", err)
	}
}

func (s *adminService) GetAdminInfo(ctx context.Context, adminID uint) (*systemVO.AdminInfoVO, error) {
	var vo *systemVO.AdminInfoVO
	key := cache.KeyAdminInfo(adminID)

	err := s.cacheMgr.Fetch(ctx, key, "admin", []string{cache.TagAdminInfo}, cache.TTL_RBAC, &vo, func() (interface{}, error) {
		admin, err := s.adminRepo.GetByID(ctx, adminID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.New(errorx.CodeUserNotFound)
			}
			slog.Error("adminRepo.GetByID failed", "adminID", adminID, "err", err)
			return nil, fmt.Errorf("adminRepo.GetByID: %w", err)
		}

		return &systemVO.AdminInfoVO{
			UserID:   strconv.FormatUint(uint64(admin.ID), 10),
			Username: admin.Username,
			Roles:    admin.RoleCodes(),
			Buttons:  admin.ButtonCodes(),
		}, nil
	})

	return vo, err
}

func (s *adminService) GetProfile(ctx context.Context, adminID uint) (*systemVO.ProfileVO, error) {
	admin, err := s.adminRepo.GetByID(ctx, adminID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeUserNotFound)
		}
		slog.Error("adminRepo.GetByID failed", "adminID", adminID, "err", err)
		return nil, fmt.Errorf("adminRepo.GetByID: %w", err)
	}

	return &systemVO.ProfileVO{
		ID:        admin.ID,
		Username:  admin.Username,
		Nickname:  admin.Nickname,
		Phone:     admin.Phone,
		Email:     admin.Email,
		Gender:    admin.Gender,
		Status:    admin.Status,
		CreatedAt: admin.CreatedAt.Format(time.DateTime),
	}, nil
}

func (s *adminService) UpdateProfile(ctx context.Context, adminID uint, req *systemDto.UpdateProfileReq) error {
	admin, err := s.adminRepo.GetByID(ctx, adminID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeUserNotFound)
		}
		slog.Error("adminRepo.GetByID failed", "adminID", adminID, "err", err)
		return fmt.Errorf("adminRepo.GetByID: %w", err)
	}

	admin.Nickname = req.Nickname
	admin.Phone = req.Phone
	admin.Email = req.Email
	admin.Gender = req.Gender

	err = s.adminRepo.Update(ctx, admin)
	if err == nil {
		// 失效管理员信息缓存，避免后台显示旧资料
		if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo); cErr != nil {
			slog.Error("invalidate cache failed", "tag", cache.TagAdminInfo, "err", cErr)
		}
	}
	return err
}

func (s *adminService) ChangePassword(ctx context.Context, adminID uint, req *systemDto.ChangePasswordReq) error {
	admin, err := s.adminRepo.GetByID(ctx, adminID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeUserNotFound)
		}
		slog.Error("adminRepo.GetByID failed", "adminID", adminID, "err", err)
		return fmt.Errorf("adminRepo.GetByID: %w", err)
	}

	if err := password.Verify(admin.Password, req.OldPassword); err != nil {
		return errorx.New(errorx.CodeOldPasswordWrong)
	}

	// 新旧密码不能相同，防止用户设置相同密码降低安全性
	if req.NewPassword == req.OldPassword {
		return errorx.New(errorx.CodeInvalidParams, "新密码不能与旧密码相同")
	}

	// 校验密码强度：必须包含大小写字母、数字、特殊符号中的至少 3 类
	if err := validateAdminPasswordStrength(req.NewPassword); err != nil {
		return err
	}

	hashedPassword, err := password.Hash(req.NewPassword)
	if err != nil {
		return errorx.New(errorx.CodeInternalError, "密码加密失败")
	}

	admin.Password = hashedPassword

	// 改密：TM 单事务原子完成「递增 token_version + 更新管理员」（fail-closed）。
	// 任一步失败整体回滚，避免「密码已改/版本号未变」或「版本号已变/密码未改」的中间态。
	// 事务提交后失效 auth_state 缓存，避免 30s TTL 窗口内旧 token 误判。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.adminRepo.IncrementTokenVersion(txCtx, adminID); err != nil {
		slog.Error("admin change password: increment token version failed", "adminID", adminID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "令牌失效失败")
	}
	if err := s.adminRepo.Update(txCtx, admin); err != nil {
		slog.Error("admin change password: update admin failed", "adminID", adminID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "管理员更新失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("admin change password: commit failed", "adminID", adminID, "err", err)
		return errorx.New(errorx.CodeInternalError, "事务提交失败")
	}
	// 事务提交后失效 auth_state 缓存
	s.invalidateAdminAuthStateCache(ctx, adminID)
	return nil
}

func (s *adminService) GetByID(ctx context.Context, id uint) (*systemEntity.Admin, error) {
	return s.adminRepo.GetByID(ctx, id)
}

func (s *adminService) GetByUsername(ctx context.Context, username string) (*systemEntity.Admin, error) {
	return s.adminRepo.GetByUsername(ctx, username)
}

func (s *adminService) ExistsByUsername(ctx context.Context, username string, excludeID ...uint) (bool, error) {
	return s.adminRepo.ExistsByUsername(ctx, username, excludeID...)
}
