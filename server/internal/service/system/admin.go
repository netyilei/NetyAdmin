// admin.go 管理员服务：接口定义、构造函数、工具函数及个人资料相关方法。
// 认证相关方法见 admin_auth.go，增删改查相关方法见 admin_manage.go。
package system

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"

	systemVO "NetyAdmin/internal/domain/vo/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/password"
	systemRepo "NetyAdmin/internal/repository/system"
	userService "NetyAdmin/internal/service/user"
)

type AdminService interface {
	Login(ctx context.Context, req *systemDto.LoginReq) (*systemVO.LoginVO, error)
	Logout(ctx context.Context, adminID uint, token string) error
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
}

func NewAdminService(adminRepo systemRepo.AdminRepository, roleRepo systemRepo.RoleRepository, jwtInstance *jwt.JWT, cacheMgr cache.LazyCacheManager, tokenStore userService.TokenStore) AdminService {
	return &adminService{
		adminRepo:  adminRepo,
		roleRepo:   roleRepo,
		jwt:        jwtInstance,
		cacheMgr:   cacheMgr,
		tokenStore: tokenStore,
	}
}

// validateAdminPasswordStrength 校验管理员密码强度。
// 委托给 pkg/password.ValidateStrength，使用管理员端默认配置（3 类字符）。
func validateAdminPasswordStrength(pwd string) error {
	if err := password.ValidateStrength(pwd, password.DefaultAdminStrengthConfig); err != nil {
		return errorx.New(errorx.CodePasswordTooWeak, err.Error())
	}
	return nil
}

// invalidateAdminTokens 全局失效管理员的所有旧 token（BUG #5 + 鉴权方案 C）。
//
// 职责切分（鉴权方案 C）：
//   - TokenVersion 专职"用户粒度的全局失效"：改密/禁用/删除时递增
//   - tokenStore 专职"单 token 粒度的精确失效"：仅登出单设备时用 Delete(单哈希)
//
// 因此本函数只递增 TokenVersion，不调 tokenStore.DeleteAll（版本号已全局兜底）。
//
// 用于 ChangePassword/Update/UpdateStatus 等非删除场景：
//   - 递增 token_version（DB 操作）
//   - 失效 auth_state 缓存（双写一致性，避免 30s TTL 窗口内旧 token 误判）
//
// Delete/DeleteBatch 不调用本函数：token_version 递增已合并到 DeleteWithTokenInvalidation 事务内，
// 事务后直接调用 invalidateAdminAuthStateCache 失效缓存。
func (s *adminService) invalidateAdminTokens(ctx context.Context, adminID uint) error {
	if err := s.adminRepo.IncrementTokenVersion(ctx, adminID); err != nil {
		return err
	}
	s.invalidateAdminAuthStateCache(ctx, adminID)
	return nil
}

// invalidateAdminAuthStateCache 失效管理员鉴权状态缓存（按 adminID 精准）。
// 用于 token_version 变更后保证下次鉴权重算，避免 30s TTL 窗口内旧 token 绕过版本号校验。
// 失败仅记录日志不阻断：DB 层 token_version 已是最终值，缓存最长 30s 后自然过期。
func (s *adminService) invalidateAdminAuthStateCache(ctx context.Context, adminID uint) {
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminAuthByID(adminID)); err != nil {
		slog.Error("invalidate admin auth_state cache failed",
			"adminID", adminID, "err", err)
	}
}

func (s *adminService) GetAdminInfo(ctx context.Context, adminID uint) (*systemVO.AdminInfoVO, error) {
	var vo *systemVO.AdminInfoVO
	key := cache.KeyAdminInfo(adminID)

	err := s.cacheMgr.Fetch(ctx, key, "admin", []string{cache.TagAdminInfo}, cache.TTL_RBAC, &vo, func() (interface{}, error) {
		admin, err := s.adminRepo.GetByID(ctx, adminID)
		if err != nil {
			return nil, errorx.New(errorx.CodeUserNotFound)
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
		return nil, errorx.New(errorx.CodeUserNotFound)
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
		return errorx.New(errorx.CodeUserNotFound)
	}

	admin.Nickname = req.Nickname
	admin.Phone = req.Phone
	admin.Email = req.Email
	admin.Gender = req.Gender

	err = s.adminRepo.Update(ctx, admin)
	if err == nil {
		// 失效管理员信息缓存，避免后台显示旧资料
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo)
	}
	return err
}

func (s *adminService) ChangePassword(ctx context.Context, adminID uint, req *systemDto.ChangePasswordReq) error {
	admin, err := s.adminRepo.GetByID(ctx, adminID)
	if err != nil {
		return errorx.New(errorx.CodeUserNotFound)
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

	// 改密码：失效旧 token + 递增版本号（fail-closed）
	if err := s.invalidateAdminTokens(ctx, adminID); err != nil {
		return errorx.New(errorx.CodeInternalError, "令牌失效失败")
	}

	return s.adminRepo.Update(ctx, admin)
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
