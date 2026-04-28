package v1

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/interface/admin/http/handler/v1/system"
)

type SystemRouter struct {
	handler *system.SystemHandler
}

func NewSystemRouter(handler *system.SystemHandler) *SystemRouter {
	return &SystemRouter{handler: handler}
}

func (r *SystemRouter) RegisterPublic(group *gin.RouterGroup) {
	// 基础设置组 (公开部分)
	system := group.Group("/system")
	{
		// 登录页需要获取验证码配置，所以获取配置接口必须公开
		system.GET("/configs", r.handler.Config.ListByGroup)
	}
}

func (r *SystemRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *SystemRouter) RegisterPermission(group *gin.RouterGroup) {
	// 1. 系统管理组 (RBAC 相关) - 对应前端 systemManage 路径
	systemManage := group.Group("/systemManage")
	{
		r.registerRoleRoutes(systemManage)
		r.registerMenuRoutes(systemManage)
		r.registerAPIRoutes(systemManage)
		r.registerButtonRoutes(systemManage)
		r.registerRolePermissionRoutes(systemManage)
	}

	// 2. 基础设置组 - 对应前端 system 路径
	system := group.Group("/system")
	{
		// 配置修改需要权限
		system.PUT("/configs", r.handler.Config.Upsert)
		// 测试邮件发送需要权限
		system.POST("/test-email", r.handler.Config.TestEmail)
	}
}

func (r *SystemRouter) registerRoleRoutes(group *gin.RouterGroup) {
	group.GET("/getRoleList", r.handler.GetAdminRoleList)
	group.GET("/getRole/:id", r.handler.GetAdminRoleByID)
	group.GET("/getAllRoles", r.handler.GetAllAdminRoles)
	group.POST("/addRole", r.handler.AddAdminRole)
	group.PUT("/updateRole", r.handler.UpdateAdminRole)
	group.DELETE("/deleteRole", r.handler.DeleteAdminRole)
	group.DELETE("/deleteRoles", r.handler.DeleteAdminRoles)
}

func (r *SystemRouter) registerMenuRoutes(group *gin.RouterGroup) {
	group.GET("/getMenuList", r.handler.GetAdminMenuList)
	group.GET("/getMenuTree", r.handler.GetAdminMenuTree)
	group.GET("/getButtonTree", r.handler.GetAdminButtonTree)
	group.GET("/getApiTree", r.handler.GetAdminApiTree)
	group.GET("/getAllPages", r.handler.GetAllPages)
	group.GET("/getMenu/:id", r.handler.GetAdminMenuByID)
	group.POST("/addMenu", r.handler.AddAdminMenu)
	group.PUT("/updateMenu", r.handler.UpdateAdminMenu)
	group.DELETE("/deleteMenu", r.handler.DeleteAdminMenu)
	group.DELETE("/deleteMenus", r.handler.DeleteAdminMenus)
}

func (r *SystemRouter) registerAPIRoutes(group *gin.RouterGroup) {
	group.GET("/getApiList", r.handler.GetAdminAPIList)
	group.GET("/getAllApi", r.handler.GetAllAdminAPI)
	group.GET("/getApi/:id", r.handler.GetAdminAPIByID)
	group.POST("/createApi", r.handler.AddAdminAPI)
	group.PUT("/updateApi", r.handler.UpdateAdminAPI)
	group.DELETE("/deleteApi/:id", r.handler.DeleteAdminAPI)
}

func (r *SystemRouter) registerButtonRoutes(group *gin.RouterGroup) {
	group.GET("/getButtonList", r.handler.GetAdminButtonList)
	group.GET("/getAllButton", r.handler.GetAllAdminButton)
	group.GET("/getButton/:id", r.handler.GetAdminButtonByID)
	group.POST("/createButton", r.handler.AddAdminButton)
	group.PUT("/updateButton", r.handler.UpdateAdminButton)
	group.DELETE("/deleteButton", r.handler.DeleteAdminButton)
}

func (r *SystemRouter) registerRolePermissionRoutes(group *gin.RouterGroup) {
	group.GET("/role/:id/menus", r.handler.GetAdminRoleMenus)
	group.PUT("/role/:id/menus", r.handler.UpdateAdminRoleMenus)
	group.GET("/role/:id/buttons", r.handler.GetAdminRoleButtons)
	group.PUT("/role/:id/buttons", r.handler.UpdateAdminRoleButtons)
	group.GET("/role/:id/apis", r.handler.GetAdminRoleAPIs)
	group.PUT("/role/:id/apis", r.handler.UpdateAdminRoleAPIs)
}
