// adminapi.go：users 管理端点（US-01——列表/禁用；PII 掩码输出）。
package adapters

import (
	"github.com/gin-gonic/gin"

	// blank import 供 swag 解析 @Failure 注解引用（errorx.Envelope——注解引用
	// 的类型必须在文件 import 图中；rbac adapters 同惯例）
	_ "github.com/netyilei/netyadmin/internal/kernel/errorx"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/users/api/keys"
	msgusers "github.com/netyilei/netyadmin/internal/modules/users/application"
)

// AdminDeps 是管理端点依赖。
type AdminDeps struct {
	Service    *msgusers.Service
	Authorizer authz.Authorizer
	OnDeny     func(*gin.Context, authn.Principal, string, string)
}

// RegisterAdminRoutes 注册 users 管理端点。
func RegisterAdminRoutes(authed gin.IRouter, deps AdminDeps) {
	authzMW := transport.AuthzMiddleware(deps.Authorizer, transport.PrincipalFrom, deps.OnDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}
	u := authed.Group("/users")
	u.GET("", secured(keys.PermUsersList, listUsersHandler(deps))...)
	u.GET("/:id", secured(keys.PermUsersList, getUserHandler(deps))...)
	u.PUT("/:id/status", secured(keys.PermUsersDisable, setStatusHandler(deps))...)
}

// @Summary 用户列表（PII 掩码）
// @Tags users-admin
// @ID admin.users.list
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} errorx.Envelope{data=transport.Page}
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/users [get]
func listUsersHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, size := transport.BindPage(c)
		list, total, err := deps.Service.ListUsers(c.Request.Context(), transport.Offset(page, size), size)
		if err != nil {
			transport.FailOf(c, err)
			return
		}
		transport.OK(c, transport.NewPage(list, int64(total), page, size))
	}
}

// @Summary 用户详情（PII 掩码）
// @Tags users-admin
// @ID admin.users.get
// @Param id path string true "用户 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/users/{id} [get]
func getUserHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := deps.Service.GetUser(c.Request.Context(), c.Param("id"))
		if err != nil {
			transport.FailOf(c, mapUserError(err))
			return
		}
		transport.OK(c, u)
	}
}

// setStatusRequest 状态请求。
type setStatusRequest struct {
	Status int16 `json:"status" validate:"required"`
}

// @Summary 用户禁用/启用
// @Tags users-admin
// @ID admin.users.disable
// @Accept json
// @Param id path string true "用户 ID"
// @Param body body setStatusRequest true "状态（1=启用 2=禁用）"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/users/{id}/status [put]
func setStatusHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req setStatusRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := deps.Service.SetStatus(c.Request.Context(), c.Param("id"), req.Status); err != nil {
			transport.FailOf(c, mapUserError(err)) // R1/D3：裸哨兵禁直传（4041 语义）
			return
		}
		transport.OK(c, gin.H{})
	}
}
