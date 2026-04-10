package system

import (
	"context"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"

	systemVO "NetyAdmin/internal/domain/vo/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	systemRepo "NetyAdmin/internal/repository/system"
)

type AdminService interface {
	Login(ctx context.Context, req *systemDto.LoginReq) (*systemVO.LoginVO, error)
	RefreshToken(ctx context.Context, refreshToken string) (*systemVO.LoginVO, error)
	GetAdminInfo(ctx context.Context, adminID uint) (*systemVO.AdminInfoVO, error)
	GetProfile(ctx context.Context, adminID uint) (*systemVO.ProfileVO, error)
	UpdateProfile(ctx context.Context, adminID uint, req *systemDto.UpdateProfileReq) error
	ChangePassword(ctx context.Context, adminID uint, req *systemDto.ChangePasswordReq) error
	List(ctx context.Context, req *systemDto.AdminQuery) ([]*systemVO.AdminItemVO, int64, error)
	Create(ctx context.Context, req *systemDto.CreateAdminReq, operatorID uint) (uint, error)
	Update(ctx context.Context, req *systemDto.UpdateAdminReq, operatorID uint) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*systemEntity.Admin, error)
	GetByUsername(ctx context.Context, username string) (*systemEntity.Admin, error)
	ExistsByUsername(ctx context.Context, username string, excludeID ...uint) (bool, error)
	DeleteBatch(ctx context.Context, ids []uint) error
}

type adminService struct {
	adminRepo systemRepo.AdminRepository
	roleRepo  systemRepo.RoleRepository
	jwt       *jwt.JWT
	cacheMgr  cache.LazyCacheManager
}

func NewAdminService(adminRepo systemRepo.AdminRepository, roleRepo systemRepo.RoleRepository, jwtInstance *jwt.JWT, cacheMgr cache.LazyCacheManager) AdminService {
	return &adminService{
		adminRepo: adminRepo,
		roleRepo:  roleRepo,
		jwt:       jwtInstance,
		cacheMgr:  cacheMgr,
	}
}

func (s *adminService) Login(ctx context.Context, req *systemDto.LoginReq) (*systemVO.LoginVO, error) {
	admin, err := s.adminRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	if !admin.IsEnabled() {
		return nil, errorx.New(errorx.CodeUserDisabled)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.Password)); err != nil {
		return nil, errorx.New(errorx.CodePasswordWrong)
	}

	now := time.Now().Format(time.DateTime)
	admin.LastLoginAt = &now
	_ = s.adminRepo.Update(ctx, admin)

	roles := admin.RoleCodes()
	token, err := s.jwt.GenerateToken(admin.ID, admin.Username, roles, jwt.AccessToken)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成令牌失败")
	}

	refreshToken, err := s.jwt.GenerateToken(admin.ID, admin.Username, roles, jwt.RefreshToken)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成刷新令牌失败")
	}

	// 注：登录成功后不再主动写入角色的硬编码 Redis，
	// 我们将在权限拦截器里使用 LazyCacheManager 进行透明加载 (Fetch)

	return &systemVO.LoginVO{
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (s *adminService) RefreshToken(ctx context.Context, refreshToken string) (*systemVO.LoginVO, error) {
	claims, err := s.jwt.ParseToken(refreshToken)
	if err != nil {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}
	if claims.Subject != string(jwt.RefreshToken) {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}

	// 检查 RefreshToken 是否在黑名单中
	blacklistKey := cache.KeyAuthBlacklistRefreshToken(refreshToken)
	exists, _ := s.cacheMgr.Exists(ctx, blacklistKey)
	if exists {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌已失效，请重新登录")
	}

	admin, err := s.adminRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}
	if !admin.IsEnabled() {
		return nil, errorx.New(errorx.CodeUserDisabled)
	}

	roles := admin.RoleCodes()
	token, err := s.jwt.GenerateToken(admin.ID, admin.Username, roles, jwt.AccessToken)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成令牌失败")
	}

	newRefreshToken, err := s.jwt.GenerateToken(admin.ID, admin.Username, roles, jwt.RefreshToken)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成刷新令牌失败")
	}

	// 将旧的 RefreshToken 标记为作废（加入黑名单，24小时过期）
	blacklistKey = cache.KeyAuthBlacklistRefreshToken(refreshToken)
	_ = s.cacheMgr.Set(ctx, blacklistKey, "1", 24*time.Hour)

	return &systemVO.LoginVO{
		Token:        token,
		RefreshToken: newRefreshToken,
	}, nil
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

	return s.adminRepo.Update(ctx, admin)
}

func (s *adminService) ChangePassword(ctx context.Context, adminID uint, req *systemDto.ChangePasswordReq) error {
	admin, err := s.adminRepo.GetByID(ctx, adminID)
	if err != nil {
		return errorx.New(errorx.CodeUserNotFound)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.OldPassword)); err != nil {
		return errorx.New(errorx.CodeOldPasswordWrong)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errorx.New(errorx.CodeInternalError, "密码加密失败")
	}

	admin.Password = string(hashedPassword)
	return s.adminRepo.Update(ctx, admin)
}

func (s *adminService) List(ctx context.Context, req *systemDto.AdminQuery) ([]*systemVO.AdminItemVO, int64, error) {
	query := &systemRepo.AdminRepoQuery{
		Username: req.Username,
		Nickname: req.Nickname,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
		Gender:   req.Gender,
		Current:  req.Current,
		Size:     req.Size,
	}

	admins, total, err := s.adminRepo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*systemVO.AdminItemVO, 0, len(admins))
	for _, a := range admins {
		var gender *string
		if a.Gender != "" {
			gender = &a.Gender
		}
		items = append(items, &systemVO.AdminItemVO{
			ID:        a.ID,
			Username:  a.Username,
			Nickname:  a.Nickname,
			Phone:     a.Phone,
			Email:     a.Email,
			Gender:    gender,
			Status:    a.Status,
			Roles:     a.RoleCodes(),
			Creator:   a.CreatorName(),
			CreatedAt: a.CreatedAt.Format(time.DateTime),
			Updater:   a.UpdaterName(),
			UpdatedAt: a.UpdatedAt.Format(time.DateTime),
		})
	}

	return items, total, nil
}

func (s *adminService) Create(ctx context.Context, req *systemDto.CreateAdminReq, operatorID uint) (uint, error) {
	exists, err := s.adminRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errorx.New(errorx.CodeUserAlreadyExists)
	}

	for _, code := range req.Roles {
		if code == systemEntity.SuperRoleCode {
			return 0, errorx.New(errorx.CodeCannotModifySuper, "不允许创建超级管理员")
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, errorx.New(errorx.CodeInternalError, "密码加密失败")
	}

	admin := &systemEntity.Admin{
		Username: req.Username,
		Password: string(hashedPassword),
		Nickname: req.Nickname,
		Phone:    req.Phone,
		Email:    req.Email,
		Gender:   req.Gender,
		Status:   req.Status,
	}
	admin.CreatedBy = operatorID

	if len(req.Roles) > 0 {
		roles, err := s.roleRepo.GetByCodes(ctx, req.Roles)
		if err != nil {
			return 0, err
		}
		admin.Roles = roles
	}

	if err := s.adminRepo.Create(ctx, admin); err != nil {
		return 0, err
	}

	return admin.ID, nil
}

func (s *adminService) Update(ctx context.Context, req *systemDto.UpdateAdminReq, operatorID uint) error {
	admin, err := s.adminRepo.GetByID(ctx, req.ID)
	if err != nil {
		return errorx.New(errorx.CodeUserNotFound)
	}

	if admin.IsSuperAdmin() {
		return errorx.New(errorx.CodeCannotModifySuper)
	}

	exists, err := s.adminRepo.ExistsByUsername(ctx, req.Username, req.ID)
	if err != nil {
		return err
	}
	if exists {
		return errorx.New(errorx.CodeUserAlreadyExists)
	}

	admin.Username = req.Username
	admin.Nickname = req.Nickname
	admin.Phone = req.Phone
	admin.Email = req.Email
	admin.Gender = req.Gender
	admin.Status = req.Status
	admin.UpdatedBy = operatorID

	if len(req.Roles) > 0 {
		roles, err := s.roleRepo.GetByCodes(ctx, req.Roles)
		if err != nil {
			return err
		}
		admin.Roles = roles
	} else {
		admin.Roles = nil
	}

	err = s.adminRepo.Update(ctx, admin)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo)
	}
	return err
}

func (s *adminService) Delete(ctx context.Context, id uint) error {
	admin, err := s.adminRepo.GetByID(ctx, id)
	if err != nil {
		return errorx.New(errorx.CodeUserNotFound)
	}

	if admin.IsSuperAdmin() {
		return errorx.New(errorx.CodeCannotDeleteSuper)
	}

	err = s.adminRepo.Delete(ctx, id)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo)
	}
	return err
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

func (s *adminService) DeleteBatch(ctx context.Context, ids []uint) error {
	for _, id := range ids {
		admin, err := s.adminRepo.GetByID(ctx, id)
		if err != nil {
			continue
		}
		if admin.IsSuperAdmin() {
			continue
		}
		_ = s.adminRepo.Delete(ctx, id)
	}
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo)
	return nil
}
