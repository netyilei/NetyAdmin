package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	userEntity "NetyAdmin/internal/domain/entity/user"
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

func (s *adminService) Login(ctx context.Context, req *systemDto.LoginReq) (*systemVO.LoginVO, error) {
	// 检查账户是否被锁定
	lockKey := cache.KeyAdminLoginLock(req.Username)
	var lockVal string
	if err := s.cacheMgr.Get(ctx, lockKey, &lockVal); err == nil && lockVal != "" {
		return nil, errorx.New(errorx.CodeUserLocked, "账户已被锁定，请稍后再试")
	}

	admin, err := s.adminRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	if !admin.IsEnabled() {
		return nil, errorx.New(errorx.CodeUserDisabled)
	}

	if err := password.Verify(admin.Password, req.Password); err != nil {
		retryKey := cache.KeyAdminLoginRetryCount(req.Username)
		var retryVal string
		var retryCount int
		if err := s.cacheMgr.Get(ctx, retryKey, &retryVal); err == nil && retryVal != "" {
			retryCount, _ = strconv.Atoi(retryVal)
		}
		retryCount++

		if retryCount >= 5 {
			_ = s.cacheMgr.Set(ctx, lockKey, "1", 15*time.Minute)
			_ = s.cacheMgr.Delete(ctx, retryKey)
			return nil, errorx.New(errorx.CodeUserLocked, "密码错误次数过多，账户已被锁定 15 分钟")
		}

		_ = s.cacheMgr.Set(ctx, retryKey, strconv.Itoa(retryCount), 10*time.Minute)
		return nil, errorx.New(errorx.CodePasswordWrong, fmt.Sprintf("密码错误，剩余尝试次数 %d 次", 5-retryCount))
	}

	// 登录成功，清除失败计数
	_ = s.cacheMgr.Delete(ctx, cache.KeyAdminLoginRetryCount(req.Username))

	now := time.Now().Format(time.DateTime)
	// 使用 UpdateColumn 仅更新 last_login_at，避免 Save 覆盖并发修改及 Preload 关联
	_ = s.adminRepo.UpdateLastLoginAt(ctx, admin.ID, now)

	roles := admin.RoleCodes()
	claims := s.jwt.NewAdminClaims(admin.ID, admin.Username, roles, jwt.AccessToken)
	token, err := s.jwt.GenerateToken(claims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成令牌失败")
	}

	refreshClaims := s.jwt.NewAdminClaims(admin.ID, admin.Username, roles, jwt.RefreshToken)
	refreshToken, err := s.jwt.GenerateToken(refreshClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成刷新令牌失败")
	}

	// 写入 token hash 到 tokenStore，使中间件可校验会话有效性，支持改密码/禁用/登出后立即失效
	userIDKey := adminTokenUserID(admin.ID)
	if s.tokenStore != nil {
		_ = s.tokenStore.Create(ctx, &userEntity.UserTokenHash{
			UserID:    userIDKey,
			TokenHash: computeAdminTokenHash(token),
			ExpiredAt: time.Unix(claims.ExpiresAt.Unix(), 0),
		})
		_ = s.tokenStore.Create(ctx, &userEntity.UserTokenHash{
			UserID:    userIDKey,
			TokenHash: computeAdminTokenHash(refreshToken),
			ExpiredAt: time.Unix(refreshClaims.ExpiresAt.Unix(), 0),
		})
	}

	// 注：登录成功后不再主动写入角色的硬编码 Redis，
	// 我们将在权限拦截器里使用 LazyCacheManager 进行透明加载 (Fetch)

	return &systemVO.LoginVO{
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (s *adminService) Logout(ctx context.Context, adminID uint, token string) error {
	if s.tokenStore == nil {
		return nil
	}
	tokenHash := computeAdminTokenHash(token)
	return s.tokenStore.Delete(ctx, adminTokenUserID(adminID), tokenHash)
}

func (s *adminService) RefreshToken(ctx context.Context, refreshToken string) (*systemVO.LoginVO, error) {
	claims := &jwt.AdminClaims{}
	if err := s.jwt.ParseToken(refreshToken, claims); err != nil {
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
	newClaims := s.jwt.NewAdminClaims(admin.ID, admin.Username, roles, jwt.AccessToken)
	token, err := s.jwt.GenerateToken(newClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成令牌失败")
	}

	refreshClaims := s.jwt.NewAdminClaims(admin.ID, admin.Username, roles, jwt.RefreshToken)
	newRefreshToken, err := s.jwt.GenerateToken(refreshClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成刷新令牌失败")
	}

	// 将旧的 RefreshToken 标记为作废（加入黑名单，24小时过期）
	blacklistKey = cache.KeyAuthBlacklistRefreshToken(refreshToken)
	_ = s.cacheMgr.Set(ctx, blacklistKey, "1", 24*time.Hour)

	// 刷新令牌：失效该管理员的所有旧 token（包括旧 AccessToken），然后写入新 access + refresh token hash
	// 这样可保证旧 AccessToken 在刷新后立即失效，防止 Token 泄露后被继续使用
	if s.tokenStore != nil {
		userIDKey := adminTokenUserID(admin.ID)
		_ = s.tokenStore.DeleteAll(ctx, userIDKey)
		_ = s.tokenStore.Create(ctx, &userEntity.UserTokenHash{
			UserID:    userIDKey,
			TokenHash: computeAdminTokenHash(token),
			ExpiredAt: time.Unix(newClaims.ExpiresAt.Unix(), 0),
		})
		_ = s.tokenStore.Create(ctx, &userEntity.UserTokenHash{
			UserID:    userIDKey,
			TokenHash: computeAdminTokenHash(newRefreshToken),
			ExpiredAt: time.Unix(refreshClaims.ExpiresAt.Unix(), 0),
		})
	}

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

func (s *adminService) Create(ctx context.Context, req *systemDto.CreateAdminReq, operatorID uint, operatorIsSuper bool) (uint, error) {
	exists, err := s.adminRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errorx.New(errorx.CodeUserAlreadyExists)
	}

	// 普通管理员不允许创建超级管理员；仅超级管理员可分配 R_SUPER 角色
	if !operatorIsSuper {
		for _, code := range req.Roles {
			if code == systemEntity.SuperRoleCode {
				return 0, errorx.New(errorx.CodeCannotModifySuper, "普通管理员无权创建超级管理员")
			}
		}
	}

	// 校验密码强度：必须包含大小写字母、数字、特殊符号中的至少 3 类
	if err := validateAdminPasswordStrength(req.Password); err != nil {
		return 0, err
	}

	hashedPassword, err := password.Hash(req.Password)
	if err != nil {
		return 0, errorx.New(errorx.CodeInternalError, "密码加密失败")
	}

	admin := &systemEntity.Admin{
		Username: req.Username,
		Password: hashedPassword,
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
		// 校验角色全部存在，部分角色不存在时拒绝创建，避免数据不一致
		if len(roles) != len(req.Roles) {
			return 0, errorx.New(errorx.CodeNotFound, "部分角色不存在")
		}
		admin.Roles = roles
	}

	if err := s.adminRepo.Create(ctx, admin); err != nil {
		return 0, err
	}

	return admin.ID, nil
}

func (s *adminService) Update(ctx context.Context, req *systemDto.UpdateAdminReq, operatorID uint, operatorIsSuper bool) error {
	// 自我保护：禁止管理员修改自己，防止误操作导致系统管理瘫痪
	if req.ID == operatorID {
		return errorx.New(errorx.CodeForbidden, "不允许修改自己的账户")
	}

	admin, err := s.adminRepo.GetByID(ctx, req.ID)
	if err != nil {
		return errorx.New(errorx.CodeUserNotFound)
	}

	// 目标已是超级管理员，拒绝修改，防止篡改超管
	if admin.IsSuperAdmin() {
		return errorx.New(errorx.CodeCannotModifySuper)
	}

	// 普通管理员不允许分配超级管理员角色，防止越权提权
	if !operatorIsSuper {
		for _, code := range req.Roles {
			if code == systemEntity.SuperRoleCode {
				return errorx.New(errorx.CodeCannotModifySuper, "普通管理员无权分配超级管理员角色")
			}
		}
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

	if req.Password != "" {
		// 校验密码强度：必须包含大小写字母、数字、特殊符号中的至少 3 类
		if err := validateAdminPasswordStrength(req.Password); err != nil {
			return err
		}
		hashedPassword, err := password.Hash(req.Password)
		if err != nil {
			return errorx.New(errorx.CodeInternalError, "密码加密失败")
		}
		admin.Password = hashedPassword
	}

	// 记录是否发生角色变更，用于后续失效 Token
	rolesChanged := false
	var newRoleIDs []uint
	if len(req.Roles) > 0 {
		roles, err := s.roleRepo.GetByCodes(ctx, req.Roles)
		if err != nil {
			return err
		}
		// 校验角色全部存在，部分角色不存在时拒绝更新
		if len(roles) != len(req.Roles) {
			return errorx.New(errorx.CodeNotFound, "部分角色不存在")
		}
		// 比较角色是否变更
		oldRoleCodes := make(map[string]bool, len(admin.Roles))
		for _, r := range admin.Roles {
			oldRoleCodes[r.Code] = true
		}
		for _, r := range roles {
			if !oldRoleCodes[r.Code] {
				rolesChanged = true
				break
			}
		}
		if len(roles) != len(admin.Roles) {
			rolesChanged = true
		}
		// 收集新角色 ID，后续用 UpdateRoles 更新 many2many 关联
		newRoleIDs = make([]uint, 0, len(roles))
		for _, r := range roles {
			newRoleIDs = append(newRoleIDs, r.ID)
		}
	}
	// 空 Roles 保持原有角色不变，避免误清空导致权限丢失

	// 先更新管理员基础字段（Save 不会更新 many2many 关联）
	// 清空 Roles 避免 Save 尝试处理关联
	admin.Roles = nil
	err = s.adminRepo.Update(ctx, admin)
	if err != nil {
		return err
	}

	// 若角色变更，使用 UpdateRoles 更新 many2many 关联
	if len(newRoleIDs) > 0 {
		if err := s.adminRepo.UpdateRoles(ctx, req.ID, newRoleIDs); err != nil {
			return err
		}
	}

	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo)
	// 若修改了密码、禁用了账户或变更了角色，强制清除该管理员所有 token
	// 角色变更后旧 Token 仍有效会导致被移除权限的管理员继续访问
	if s.tokenStore != nil && (req.Password != "" || admin.Status != entity.StatusEnabled || rolesChanged) {
		_ = s.tokenStore.DeleteAll(ctx, adminTokenUserID(req.ID))
	}
	return nil
}

func (s *adminService) Delete(ctx context.Context, id uint, operatorID uint) error {
	// 自我保护：禁止管理员删除自己，防止误操作导致系统管理瘫痪
	if id == operatorID {
		return errorx.New(errorx.CodeForbidden, "不允许删除自己的账户")
	}

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
		// 删除管理员后，清除其所有 token
		if s.tokenStore != nil {
			_ = s.tokenStore.DeleteAll(ctx, adminTokenUserID(id))
		}
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

func (s *adminService) DeleteBatch(ctx context.Context, ids []uint, operatorID uint) error {
	var errs []string
	for _, id := range ids {
		// 自我保护：跳过删除自己
		if id == operatorID {
			errs = append(errs, fmt.Sprintf("管理员 %d：不允许删除自己", id))
			continue
		}
		admin, err := s.adminRepo.GetByID(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Sprintf("管理员 %d：不存在", id))
			continue
		}
		if admin.IsSuperAdmin() {
			errs = append(errs, fmt.Sprintf("管理员 %d：不允许删除超级管理员", id))
			continue
		}
		if err := s.adminRepo.Delete(ctx, id); err != nil {
			errs = append(errs, fmt.Sprintf("管理员 %d：%s", id, err.Error()))
			continue
		}
		// 批量删除后，清除对应管理员所有 token
		if s.tokenStore != nil {
			_ = s.tokenStore.DeleteAll(ctx, adminTokenUserID(id))
		}
	}
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo)

	// 收集错误并返回聚合错误，避免静默吞错导致数据不一致
	if len(errs) > 0 {
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("部分管理员删除失败：%s", strings.Join(errs, "; ")))
	}
	return nil
}
