// adminapi_roles.go：角色管理与授权绑定端点（V1 role 13 端点 RESTful
// 收敛——CRUD 5 + 三面授权 GET/PUT 6 + admin 角色分配 GET/PUT 2 + route-tree）。
// 授权绑定全量替换语义（树勾选保存）；SetRoleApis/SetAdminRoles 发布 reload。
package adapters

import (
	"strconv"

	"github.com/gin-gonic/gin"

	// swag 注解解析需要（@Failure errorx.Envelope 类型解析——Go 编译面仅副作用无）
	_ "github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/rbac/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/rbac/domain"
)

// registerRoleRoutes 注册角色域路由（group 由 RegisterAdminRoutes 传入的 rg 承载）。
func registerRoleRoutes(rg gin.IRouter, deps Deps, secured securedChain) {
	roles := rg.Group("/roles")
	roles.GET("", secured(keys.PermRolesList, listRolesHandler(deps))...)
	roles.POST("", secured(keys.PermRolesCrea, createRoleHandler(deps))...)
	roles.GET("/:id", secured(keys.PermRolesGet, getRoleHandler(deps))...)
	roles.PUT("/:id", secured(keys.PermRolesUpd, updateRoleHandler(deps))...)
	roles.DELETE("/:id", secured(keys.PermRolesDel, deleteRoleHandler(deps))...)

	roles.GET("/:id/grants", secured(keys.PermRolesGet, roleGrantsHandler(deps))...)
	roles.PUT("/:id/menus", secured(keys.PermRolesBind, setRoleMenusHandler(deps))...)
	roles.PUT("/:id/apis", secured(keys.PermRolesBind, setRoleApisHandler(deps))...)
	roles.PUT("/:id/buttons", secured(keys.PermRolesBind, setRoleButtonsHandler(deps))...)

	// 管理员角色分配（rbac 持绑定数据，admin ID 跨模块文本引用——§9 无 FK）。
	admins := rg.Group("/admins")
	admins.GET("/:id/roles", secured(keys.PermRolesGet, adminRolesHandler(deps))...)
	admins.PUT("/:id/roles", secured(keys.PermRolesBind, setAdminRolesHandler(deps))...)

	// 管理员可见菜单树（自身导航面——登录即可，无权限码声明）。
	rg.GET("/route-tree", routeTreeHandler(deps))
}

// @Summary 角色列表
// @ID admin.rbac.roles.list
// @Tags roles
// @Produce json
// @Param code query string false "角色码包含过滤"
// @Param name query string false "名称包含过滤"
// @Param status query int false "状态过滤（1=enabled 2=disabled）"
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} transport.Page
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/roles [get]
func listRolesHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, size := transport.BindPage(c)
		status := 0
		if v, err := strconv.Atoi(c.Query("status")); err == nil {
			status = v
		}
		list, total, err := deps.Service.ListRoles(c.Request.Context(), domain.RoleListFilter{
			Code:   c.Query("code"),
			Name:   c.Query("name"),
			Status: domain.Status(status),
			Offset: transport.Offset(page, size),
			Limit:  size,
		})
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, transport.NewPage(list, total, page, size))
	}
}

// @Summary 角色详情
// @ID admin.rbac.roles.get
// @Tags roles
// @Param id path string true "角色 ID"
// @Success 200 {object} domain.Role
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/roles/{id} [get]
func getRoleHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, err := deps.Service.GetRole(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, role)
	}
}

type roleRequest struct {
	Code   string `json:"code" validate:"required,min=2,max=64"`
	Name   string `json:"name" validate:"required,max=64"`
	Status int    `json:"status" validate:"omitempty,eq=1|eq=2"`
	Remark string `json:"remark" validate:"max=256"`
}

// @Summary 创建角色
// @ID admin.rbac.roles.create
// @Tags roles
// @Accept json
// @Param body body roleRequest true "角色定义"
// @Success 200 {object} domain.Role
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/roles [post]
func createRoleHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req roleRequest
		if !transport.Bind(c, &req) {
			return
		}
		role, err := deps.Service.CreateRole(c.Request.Context(), domain.Role{
			Code: req.Code, Name: req.Name, Status: domain.Status(req.Status), Remark: req.Remark,
		})
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, role)
	}
}

func updateRoleHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req roleRequest
		if !transport.Bind(c, &req) {
			return
		}
		role, err := deps.Service.UpdateRole(c.Request.Context(), domain.Role{
			ID: c.Param("id"), Code: req.Code, Name: req.Name,
			Status: domain.Status(req.Status), Remark: req.Remark,
		})
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, role)
	}
}

// @Summary 删除角色
// @ID admin.rbac.roles.delete
// @Description 级联清理授权行 + 策略 reload
// @Tags roles
// @Param id path string true "角色 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/roles/{id} [delete]
func deleteRoleHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.DeleteRole(c.Request.Context(), c.Param("id")); err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

type grantsResult struct {
	Menus   []string `json:"menus"`
	Apis    []string `json:"apis"`
	Buttons []string `json:"buttons"`
}

// @Summary 角色授权回显
// @ID admin.rbac.roles.grants_get
// @Description 三面授权 ID 集（menus/apis/buttons）
// @Tags roles
// @Param id path string true "角色 ID"
// @Success 200 {object} grantsResult
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/roles/{id}/grants [get]
func roleGrantsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		menus, apis, buttons, err := deps.Service.RoleGrants(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, grantsResult{Menus: menus, Apis: apis, Buttons: buttons})
	}
}

// @Summary 替换角色菜单授权
// @ID admin.rbac.roles.menus_set
// @Tags roles
// @Accept json
// @Param id path string true "角色 ID"
// @Param body body idsRequest true "菜单 ID 全集"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/roles/{id}/menus [put]
func setRoleMenusHandler(deps Deps) gin.HandlerFunc {
	return idsPut(deps, func(c *gin.Context, id string, ids []string) error {
		return deps.Service.SetRoleMenus(c.Request.Context(), id, ids)
	})
}

// @Summary 替换角色 API 授权（判定面——发布策略 reload）
// @ID admin.rbac.roles.apis_set
// @Tags roles
// @Accept json
// @Param id path string true "角色 ID"
// @Param body body idsRequest true "API ID 全集"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/roles/{id}/apis [put]
func setRoleApisHandler(deps Deps) gin.HandlerFunc {
	return idsPut(deps, func(c *gin.Context, id string, ids []string) error {
		return deps.Service.SetRoleApis(c.Request.Context(), id, ids)
	})
}

// @Summary 替换角色按钮授权
// @ID admin.rbac.roles.buttons_set
// @Tags roles
// @Accept json
// @Param id path string true "角色 ID"
// @Param body body idsRequest true "按钮 ID 全集"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/roles/{id}/buttons [put]
func setRoleButtonsHandler(deps Deps) gin.HandlerFunc {
	return idsPut(deps, func(c *gin.Context, id string, ids []string) error {
		return deps.Service.SetRoleButtons(c.Request.Context(), id, ids)
	})
}

type roleIDsResult struct {
	Roles []string `json:"roles"`
}

// @Summary 管理员已绑角色
// @ID admin.rbac.admins.roles_get
// @Tags roles
// @Param id path string true "管理员 ID"
// @Success 200 {object} roleIDsResult
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/admins/{id}/roles [get]
func adminRolesHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ids, err := deps.Service.AdminRoles(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, roleIDsResult{Roles: ids})
	}
}

// @Summary 分配管理员角色（判定面——发布策略 reload）
// @ID admin.rbac.admins.roles_set
// @Tags roles
// @Accept json
// @Param id path string true "管理员 ID"
// @Param body body idsRequest true "角色 ID 全集"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/admins/{id}/roles [put]
func setAdminRolesHandler(deps Deps) gin.HandlerFunc {
	return idsPut(deps, func(c *gin.Context, id string, ids []string) error {
		p := transport.PrincipalFrom(c)
		return deps.Service.SetAdminRoles(c.Request.Context(), id, p.ID, ids) // operatorID=自操作防呆
	})
}

// @Summary 当前管理员权限码集（按钮权限/指令消费）
// @Description super-admin=全部登记 act；否则 role_apis 并集
// @Tags roles
// @ID admin.rbac.permissions
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /admin/v1/permissions [get]
func permissionsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		acts, err := deps.Service.PermissionActs(c.Request.Context(), p.ID)
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, gin.H{"acts": acts})
	}
}

// @Summary 管理员可见菜单树
// @ID admin.rbac.route.tree
// @Description super-admin 全量启用菜单；否则按 role_menus 并集
// @Tags roles
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /admin/v1/route-tree [get]
func routeTreeHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		menus, err := deps.Service.RouteTree(c.Request.Context(), p.ID)
		if err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, menus)
	}
}

// —— 共用小件 ——

// idsRequest 是授权替换请求体（PUT 全量替换语义——缺省/空数组=清空授权集）。
type idsRequest struct {
	IDs []string `json:"ids" validate:"dive,required"`
}

// idsPut 统一「替换授权」端点形态（PUT /:id/<face>）。
func idsPut(deps Deps, apply func(c *gin.Context, id string, ids []string) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req idsRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := apply(c, c.Param("id"), req.IDs); err != nil {
			writeRbacError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}
