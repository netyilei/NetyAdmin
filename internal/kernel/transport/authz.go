// authz.go：端点级授权中间件（权限链；Route policy——
// 「路由元数据声明所需权限」）。判定语义：enforce(subject, 路由模板, 权限码)，
// 权限码由路由经 RequirePerm 声明（rbac_apis.act 数据契约同源，super-admin
// 通配行 act='*' 全通过）。 deny → OnDeny 回调（装配方接审计）+ 10106。
// PrincipalFrom 是 gin context 取 Principal 的统一出口（authRequired 注入方）。
package transport

import (
	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
)

// CtxPermKey 是权限码在 gin context 的键（RequirePerm 写、AuthzMiddleware 读）。
const CtxPermKey = "perm_code"

// PrincipalFrom 取 authRequired 注入的 Principal（缺失返回零值——调用方按需判定）。
func PrincipalFrom(c *gin.Context) authn.Principal {
	if v, ok := c.Get("principal"); ok {
		if p, ok := v.(authn.Principal); ok {
			return p
		}
	}
	return authn.Principal{}
}

// RequirePerm 声明路由所需权限码（Route policy 元数据；必须位于 AuthzMiddleware
// 之前执行——链序 RequirePerm → AuthzMiddleware → handler）。
func RequirePerm(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(CtxPermKey, code)
		c.Next()
	}
}

// Secured 组装受保护路由链（RequirePerm 声明 → authz 判定 → handler；顺序
// 敏感——声明必须先于判定。identity/rbac/audit 三模块同构链收拢——G1-06）。
func Secured(authzMw gin.HandlerFunc, perm string, h gin.HandlerFunc) []gin.HandlerFunc {
	return []gin.HandlerFunc{RequirePerm(perm), authzMw, h}
}

// AuthzMiddleware 端点级 RBAC 判定（admin 主体）。路由未声明权限码 = 登录即可
// （profile/route-tree 类自身面）；声明了权限码而 authorizer 为 nil = fail-closed
// 全拒（装配缺失不得静默放行——限流 nil 直通教训对偶， 核对项）。
// Authorize 系统错误按 fail-closed 拒绝（authz 契约：错误=系统故障）。
// onDeny 为审计回调（可 nil）：装配方写 SecurityRecord，中间件不依赖审计模块。
func AuthzMiddleware(author authz.Authorizer,
	principalFn func(*gin.Context) authn.Principal,
	onDeny func(c *gin.Context, p authn.Principal, perm, reason string)) gin.HandlerFunc {

	return func(c *gin.Context) {
		perm := c.GetString(CtxPermKey)
		if perm == "" { // 路由自身未声明权限码：登录即可访问
			c.Next()
			return
		}
		p := principalFn(c)
		d := authz.Decision{Allowed: false, Reason: authz.ReasonNotWired}
		if author != nil {
			var err error
			d, err = author.Authorize(c.Request.Context(), p, c.FullPath(), perm)
			if err != nil {
				if onDeny != nil {
					onDeny(c, p, perm, authz.ReasonSystemError)
				}
				FailOf(c, err)
				return
			}
		}
		if !d.Allowed {
			if onDeny != nil {
				onDeny(c, p, perm, d.Reason)
			}
			Fail(c, errorx.New(errorx.CodePermissionDenied))
			return
		}
		c.Next()
	}
}
