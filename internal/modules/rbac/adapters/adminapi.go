// adminapi.go：RBAC 管理 HTTP 端点——rbac 模块的 HTTP 适配（gin
// handlers 调 application.Service）。模块自注册模式：cmd 组装时经
// Module.RegisterAdminRoutes 挂到 adminapi 版本组，adminapi 门不 import 本模块。
// 链序：authRequired（authn，cmd/identity 面已判）→ AuthzMiddleware →
// RequirePerm → handler。写路径发布 platform.rbac.changed（application 编排）。
package adapters

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/rbac/application"
	"github.com/netyilei/netyadmin/internal/modules/rbac/domain"
)

// Deps 是 handlers 依赖（module.go 装配注入）。
type Deps struct {
	Service    *application.Service
	Authorizer authz.Authorizer // RBAC 判定面（cmd 注入；nil=fail-closed 全拒）
	OnDeny     func(*gin.Context, authn.Principal, string, string)
}

// RegisterAdminRoutes 注册 RBAC 管理端点（module.go 调用；子文件按资源拆分）。
func RegisterAdminRoutes(rg gin.IRouter, deps Deps) {
	authzMW := transport.AuthzMiddleware(deps.Authorizer, transport.PrincipalFrom, deps.OnDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}
	registerRoleRoutes(rg, deps, secured)
	registerMenuRoutes(rg, deps, secured)
	registerApiRoutes(rg, deps, secured)
	registerButtonRoutes(rg, deps, secured)
	// permissions=自查面（登录用户查自身权限码集——无 Perm 守卫； 前端按钮权限）
	rg.GET("/permissions", permissionsHandler(deps))
}

// secured 链类型（子文件共用签名）。
type securedChain func(perm string, h gin.HandlerFunc) []gin.HandlerFunc

// writeRbacError 领域错误映射（errorx 信封）：code 冲突 → 10004 conflict；
// 不存在 → 10003；超管保护/绑定已存在 → 10006 forbidden。
func writeRbacError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrCodeTaken):
		transport.Fail(c, errorx.New(errorx.CodeConflict, "reason", "code taken"))
	case errors.Is(err, domain.ErrSuperAdminProtected), errors.Is(err, domain.ErrAlreadyBound):
		transport.Fail(c, errorx.New(errorx.CodeForbidden))
	case errors.Is(err, domain.ErrRoleNotFound), errors.Is(err, domain.ErrNotFound):
		transport.Fail(c, errorx.New(errorx.CodeNotFound))
	default:
		transport.FailOf(c, err)
	}
}
