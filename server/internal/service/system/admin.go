// admin.go 管理员服务：接口定义、构造函数、工具函数及个人资料相关方法。
// 认证相关方法见 admin_auth.go，增删改查相关方法见 admin_manage.go。
package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
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

// adminTokenUserID 将 adminID（uint）转为 tokenStore 所需的 string 形式用户标识
func adminTokenUserID(adminID uint) string {
	return "a:" + strconv.FormatUint(uint64(adminID), 10)
}

// computeAdminTokenHash 计算 admin token 的 sha256 哈希，与中间件校验保持一致
func computeAdminTokenHash(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// validateAdminPasswordStrength 校验管理员密码强度：必须包含大小写字母、数字、特殊符号中的至少 3 类
func validateAdminPasswordStrength(pwd string) error {
	types := 0
	if matched, _ := regexp.MatchString(`[a-z]`, pwd); matched {
		types++
	}
	if matched, _ := regexp.MatchString(`[A-Z]`, pwd); matched {
		types++
	}
	if matched, _ := regexp.MatchString(`[0-9]`, pwd); matched {
		types++
	}
	if matched, _ := regexp.MatchString(`[^a-zA-Z0-9]`, pwd); matched {
		types++
	}
	if types < 3 {
		return errorx.New(errorx.CodePasswordTooWeak, "密码必须包含大小写字母、数字、特殊符号中的至少 3 种")
	}
	return nil
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

	// 改密码后强制清除该管理员所有 token，使旧 access/refresh token 立即失效
	if s.tokenStore != nil {
		_ = s.tokenStore.DeleteAll(ctx, adminTokenUserID(adminID))
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
