// admin_manage.go 管理员管理：列表、创建、更新、删除、批量删除。
package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemVO "NetyAdmin/internal/domain/vo/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"

	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/password"
	systemRepo "NetyAdmin/internal/repository/system"
)

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
	// 若修改了密码、禁用了账户或变更了角色，强制失效该管理员所有 token
	// 角色变更后旧 Token 仍有效会导致被移除权限的管理员继续访问
	if req.Password != "" || admin.Status != entity.StatusEnabled || rolesChanged {
		if err := s.invalidateAdminTokens(ctx, req.ID); err != nil {
			return errorx.New(errorx.CodeInternalError, "令牌失效失败")
		}
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

	// 单事务原子完成「清理角色关联 + 递增 token_version + 软删除」，与 user 侧语义一致。
	// 避免旧的 invalidateAdminTokens + adminRepo.Delete 两步分离导致的中间态。
	if err := s.adminRepo.DeleteWithTokenInvalidation(ctx, id); err != nil {
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败（事务回滚）：%s", id, err.Error()))
	}
	// 事务提交后失效缓存：auth_state（按 adminID 精准）+ admin:info（全局，列表/详情页可能引用）
	s.invalidateAdminAuthStateCache(ctx, id)
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo)
	return nil
}

func (s *adminService) DeleteBatch(ctx context.Context, ids []uint, operatorID uint) error {
	// 逐条事务 fail-closed：任一 id 事务失败立即返回错误（已提交的 id 保持删除状态）。
	// 业务规则拒绝（自我保护/超管保护/不存在）走 continue 跳过并记录，不阻断整个批量。
	//
	// 设计权衡（vs 旧 fail-open 实现）：
	//   - 旧实现：invalidateAdminTokens 与 Delete 分离，Inc 失败仅记录 errs 不阻断，
	//     可能出现"已删但版本号未递增"的中间态（旧 token 仍能通过版本号校验）
	//   - 新实现：单事务原子保证，事务失败立即返回；业务规则拒绝仍 continue
	var skipped []string
	for _, id := range ids {
		if id == operatorID {
			skipped = append(skipped, fmt.Sprintf("管理员 %d：不允许删除自己", id))
			continue
		}
		admin, err := s.adminRepo.GetByID(ctx, id)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("管理员 %d：不存在", id))
			continue
		}
		if admin.IsSuperAdmin() {
			skipped = append(skipped, fmt.Sprintf("管理员 %d：不允许删除超级管理员", id))
			continue
		}
		if err := s.adminRepo.DeleteWithTokenInvalidation(ctx, id); err != nil {
			// 事务失败立即返回（fail-closed）：已提交的 id 保持删除状态，未处理的 id 不受影响
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败（事务回滚）：%s", id, err.Error()))
		}
		s.invalidateAdminAuthStateCache(ctx, id)
	}
	// 全部处理完成后，全局失效 admin:info 缓存（列表页/详情页可能引用了已删 admin）
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo)
	if len(skipped) > 0 {
		return errorx.New(errorx.CodeForbidden, fmt.Sprintf("部分管理员被跳过：%s", strings.Join(skipped, "; ")))
	}
	return nil
}
