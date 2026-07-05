package system

import (
	"context"
	"strings"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/cache"
)

// 本文件包含 RoleService 的路由匹配与 API 鉴权方法。
// 从 role.go 拆分而来，降低单文件行数（B6-2 / RULES.md §1）。

// matchPath 复用 Gin 的路由匹配逻辑进行高精度匹配。
// 支持参数占位符（:param）和通配符（*action）。
func matchPath(pattern, path string) bool {
	if pattern == path {
		return true
	}

	// 如果不包含参数占位符，直接返回不匹配
	if !strings.Contains(pattern, ":") && !strings.Contains(pattern, "*") {
		return false
	}

	// 动态路由匹配：如 /admin/v1/user/:id 匹配 /admin/v1/user/123
	// 改进后的安全切片匹配算法：
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	// 如果段数不同，且没有 * 通配符，则绝对不匹配
	if len(patternParts) != len(pathParts) && !strings.Contains(pattern, "*") {
		return false
	}

	for i := 0; i < len(patternParts); i++ {
		pPattern := patternParts[i]

		// 如果遇到 * 通配符 (如 /*action)，直接匹配后续所有路径
		if strings.HasPrefix(pPattern, "*") {
			return true
		}

		// 防止越界
		if i >= len(pathParts) {
			return false
		}

		pPath := pathParts[i]

		// 遇到 : 占位符，跳过当前段的比对
		if strings.HasPrefix(pPattern, ":") {
			continue
		}

		if pPattern != pPath {
			return false
		}
	}

	// 确保 pathParts 也匹配完了 (除非有 * 号)
	if len(patternParts) != len(pathParts) {
		return false
	}

	return true
}

// VerifyApiAuth 校验指定角色集合是否拥有访问指定 API 的权限。
// 返回值：
//   - hasPermission: 角色是否拥有该 API 的访问权限
//   - apiFound: 该 API 是否存在于 API 记录中
//   - err: 内部错误（如缓存/DB 故障）
func (s *roleService) VerifyApiAuth(ctx context.Context, method, path string, roleCodes []string) (hasPermission bool, apiFound bool, err error) {
	// 1. Fetch 全部 API
	var allApis []*systemEntity.API
	err = s.cacheMgr.Fetch(ctx, cache.KeyAllApis(), "rbac_auth", []string{cache.TagRBACAPI}, cache.TTL_RBAC, &allApis, func() (interface{}, error) {
		return s.apiRepo.GetAll(ctx)
	})
	if err != nil {
		return false, false, err
	}

	var targetAPI *systemEntity.API
	for _, api := range allApis {
		if api.Method == method && matchPath(api.Path, path) {
			targetAPI = api
			break
		}
	}

	if targetAPI == nil {
		return false, false, nil // API 不存在于记录
	}

	if targetAPI.Auth == systemEntity.APINotRequireAuth {
		return true, true, nil // API 存在且免鉴权
	}

	for _, roleCode := range roleCodes {
		if roleCode == systemEntity.SuperRoleCode {
			return true, true, nil
		}
	}

	// 2. 依次 Fetch 每一个角色的拥有的 API 列表
	for _, roleCode := range roleCodes {
		var apis []*systemEntity.API
		key := cache.KeyRoleApis(roleCode)
		err = s.cacheMgr.Fetch(ctx, key, "rbac_auth", []string{cache.TagRBACRole}, cache.TTL_RBAC, &apis, func() (interface{}, error) {
			role, repoErr := s.roleRepo.GetByCode(ctx, roleCode)
			if repoErr != nil {
				return nil, repoErr
			}
			return role.Apis, nil
		})

		if err != nil {
			continue // 该角色查不到或出错，跳过检查其他角色
		}

		for _, api := range apis {
			if api.Method == method && matchPath(api.Path, path) {
				return true, true, nil
			}
		}
	}

	return false, true, nil
}
