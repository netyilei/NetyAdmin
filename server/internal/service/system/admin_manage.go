// admin_manage.go 管理员管理：列表、创建、更新、删除、批量删除。
package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeUserNotFound)
		}
		slog.Error("adminRepo.GetByID failed", "adminID", req.ID, "err", err)
		return fmt.Errorf("adminRepo.GetByID: %w", err)
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
		return fmt.Errorf("adminRepo.ExistsByUsername: %w", err)
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
			return fmt.Errorf("roleRepo.GetByCodes: %w", err)
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

	// 敏感字段变更判定（密码/状态禁用/角色变更）
	sensitiveChanged := req.Password != "" || admin.Status != entity.StatusEnabled || rolesChanged

	// 清空 Roles 避免 Save 尝试处理关联（Save 仅更新主数据）
	admin.Roles = nil

	if sensitiveChanged {
		// 敏感分支：TM 单事务原子完成「递增 token_version + Save 主数据 + 更新角色关联」（fail-closed）。
		// 任一步失败整体回滚，避免「密码已改/版本号未变」或「角色已变/版本号未变」的中间态。
		txCtx, tx := s.tm.Begin(ctx)
		if err := s.adminRepo.IncrementTokenVersion(txCtx, req.ID); err != nil {
			slog.Error("admin update: increment token version failed", "adminID", req.ID, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "管理员更新失败")
		}
		if err := s.adminRepo.Update(txCtx, admin); err != nil {
			slog.Error("admin update: save admin failed", "adminID", req.ID, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "管理员更新失败")
		}
		// 若角色变更，在同一事务内更新 many2many 关联
		if len(newRoleIDs) > 0 {
			if err := s.adminRepo.UpdateRoles(txCtx, req.ID, newRoleIDs); err != nil {
				slog.Error("admin update: update roles failed", "adminID", req.ID, "err", err)
				s.tm.Rollback(tx)
				return errorx.New(errorx.CodeInternalError, "管理员更新失败")
			}
		}
		if err := s.tm.Commit(tx); err != nil {
			slog.Error("admin update: commit failed", "adminID", req.ID, "err", err)
			return errorx.New(errorx.CodeInternalError, "事务提交失败")
		}
	} else {
		// 非敏感字段更新：TM 单事务原子完成「更新主数据 + 更新角色关联」（fail-closed）。
		// 任一步失败整体回滚，避免「基础信息已改但角色未变」的中间态。
		txCtx, tx := s.tm.Begin(ctx)
		if err := s.adminRepo.Update(txCtx, admin); err != nil {
			slog.Error("admin update: save admin failed", "adminID", req.ID, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "管理员更新失败")
		}
		if len(newRoleIDs) > 0 {
			if err := s.adminRepo.UpdateRoles(txCtx, req.ID, newRoleIDs); err != nil {
				slog.Error("admin update: update roles failed", "adminID", req.ID, "err", err)
				s.tm.Rollback(tx)
				return errorx.New(errorx.CodeInternalError, "管理员更新失败")
			}
		}
		if err := s.tm.Commit(tx); err != nil {
			slog.Error("admin update: commit failed", "adminID", req.ID, "err", err)
			return errorx.New(errorx.CodeInternalError, "事务提交失败")
		}
	}

	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagAdminInfo); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagAdminInfo, "err", err)
	}
	// 敏感分支事务已递增 TokenVersion，事务后失效 auth_state 缓存
	if sensitiveChanged {
		s.invalidateAdminAuthStateCache(ctx, req.ID)
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeUserNotFound)
		}
		slog.Error("adminRepo.GetByID failed", "adminID", id, "err", err)
		return fmt.Errorf("adminRepo.GetByID: %w", err)
	}

	if admin.IsSuperAdmin() {
		return errorx.New(errorx.CodeCannotDeleteSuper)
	}

	// TM 单事务原子完成「清理角色关联 + 递增 token_version + 软删除」。
	// 任一步失败整体回滚，避免「角色关联已清/版本号已变但主数据未删」的中间态。
	// 事务提交后失效缓存：auth_state（按 adminID 精准）+ admin:info（全局，列表/详情页可能引用）。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.adminRepo.ClearRoles(txCtx, id); err != nil {
		slog.Error("admin delete: clear roles failed", "adminID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败", id))
	}
	if err := s.adminRepo.IncrementTokenVersion(txCtx, id); err != nil {
		slog.Error("admin delete: increment token version failed", "adminID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败", id))
	}
	if err := s.adminRepo.Delete(txCtx, id); err != nil {
		slog.Error("admin delete: delete admin failed", "adminID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败", id))
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("admin delete: commit failed", "adminID", id, "err", err)
		return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败", id))
	}
	s.invalidateAdminAuthStateCache(ctx, id)
	s.invalidateAdminInfoCache(ctx)
	return nil
}

func (s *adminService) DeleteBatch(ctx context.Context, ids []uint, operatorID uint) error {
	// 逐条事务 fail-closed：任一 id 事务失败立即返回错误（已提交的 id 保持删除状态）。
	// 业务规则拒绝（自我保护/超管保护/不存在）走 continue 跳过并记录，不阻断整个批量。
	//
	// 设计权衡（vs 旧 fail-open 实现）：
	//   - 旧实现：IncrementTokenVersion 与 Delete 分离，Inc 失败仅记录 errs 不阻断，
	//     可能出现"已删但版本号未递增"的中间态（旧 token 仍能通过版本号校验）
	//   - 新实现：TM 单事务原子保证，事务失败立即返回；业务规则拒绝仍 continue
	var skipped []string
	for _, id := range ids {
		if id == operatorID {
			skipped = append(skipped, fmt.Sprintf("管理员 %d：不允许删除自己", id))
			continue
		}
		admin, err := s.adminRepo.GetByID(ctx, id)
		if err != nil {
			// 业务规则：不存在的 ID 跳过（fail-closed 语义仅针对事务失败）
			if errors.Is(err, gorm.ErrRecordNotFound) {
				skipped = append(skipped, fmt.Sprintf("管理员 %d：不存在", id))
				continue
			}
			// DB 错误（非 record not found）：fail-closed 立即返回，已删除的 id 保持，未处理的不受影响
			slog.Error("admin delete batch: GetByID failed", "adminID", id, "err", err)
			s.invalidateAdminInfoCache(ctx)
			return fmt.Errorf("adminRepo.GetByID (id=%d): %w", id, err)
		}
		if admin.IsSuperAdmin() {
			skipped = append(skipped, fmt.Sprintf("管理员 %d：不允许删除超级管理员", id))
			continue
		}
		// TM 单事务原子完成「清理角色关联 + 递增 token_version + 软删除」
		txCtx, tx := s.tm.Begin(ctx)
		if err := s.adminRepo.ClearRoles(txCtx, id); err != nil {
			slog.Error("admin delete batch: clear roles failed", "adminID", id, "err", err)
			s.tm.Rollback(tx)
			// 事务失败立即返回（fail-closed）：已提交的 id 保持删除状态，未处理的 id 不受影响
			// 但已成功删除的 id 会导致列表/详情页缓存过期，必须失效 TagAdminInfo
			s.invalidateAdminInfoCache(ctx)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败", id))
		}
		if err := s.adminRepo.IncrementTokenVersion(txCtx, id); err != nil {
			slog.Error("admin delete batch: increment token version failed", "adminID", id, "err", err)
			s.tm.Rollback(tx)
			s.invalidateAdminInfoCache(ctx)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败", id))
		}
		if err := s.adminRepo.Delete(txCtx, id); err != nil {
			slog.Error("admin delete batch: delete admin failed", "adminID", id, "err", err)
			s.tm.Rollback(tx)
			s.invalidateAdminInfoCache(ctx)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败", id))
		}
		if err := s.tm.Commit(tx); err != nil {
			slog.Error("admin delete batch: commit failed", "adminID", id, "err", err)
			s.invalidateAdminInfoCache(ctx)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("管理员 %d 删除失败", id))
		}
		s.invalidateAdminAuthStateCache(ctx, id)
	}
	// 全部处理完成后，全局失效 admin:info 缓存（列表页/详情页可能引用了已删 admin）
	s.invalidateAdminInfoCache(ctx)
	if len(skipped) > 0 {
		return errorx.New(errorx.CodeForbidden, fmt.Sprintf("部分管理员被跳过：%s", strings.Join(skipped, "; ")))
	}
	return nil
}
