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
		// 字典数据获取通常也是公开的 (例如前端根据 code 获取枚举显示)
		system.GET("/dict/data/:code", r.handler.Dict.GetDictData)
	}
}

func (r *SystemRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *SystemRouter) RegisterPermission(group *gin.RouterGroup) {
	// 1. 系统管理组 (RBAC 相关) - 对应前端 systemManage 路径
	systemManage := group.Group("/systemManage")
	{
		r.registerAdminRoutes(systemManage)
		r.registerRoleRoutes(systemManage)
		r.registerMenuRoutes(systemManage)
		r.registerAPIRoutes(systemManage)
		r.registerButtonRoutes(systemManage)
		r.registerRolePermissionRoutes(systemManage)
	}

	// 2. 基础设置与任务组 - 对应前端 system 路径
	system := group.Group("/system")
	{
		// 配置修改需要权限
		system.PUT("/configs", r.handler.Config.Upsert)

		r.registerTaskRoutes(system)
		r.registerDictAdminRoutes(system)
	}
}

func (r *SystemRouter) registerDictAdminRoutes(group *gin.RouterGroup) {
	// 管理接口
	group.GET("/dict/types", r.handler.Dict.ListType)
	group.POST("/dict/types", r.handler.Dict.CreateType)
	group.PUT("/dict/types", r.handler.Dict.UpdateType)
	group.DELETE("/dict/types/:id", r.handler.Dict.DeleteType)

	group.GET("/dict/data", r.handler.Dict.ListDataFull)
	group.POST("/dict/data", r.handler.Dict.CreateData)
	group.PUT("/dict/data", r.handler.Dict.UpdateData)
	group.DELETE("/dict/data/:id", r.handler.Dict.DeleteData)
}

func (r *SystemRouter) registerAdminRoutes(group *gin.RouterGroup) {
	group.GET("/getUserList", r.handler.GetAdminList)
	group.POST("/addUser", r.handler.AddAdmin)
	group.PUT("/updateUser", r.handler.UpdateAdmin)
	group.DELETE("/deleteUser", r.handler.DeleteAdmin)
	group.DELETE("/deleteUsers", r.handler.DeleteAdmins)
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

func (r *SystemRouter) registerTaskRoutes(group *gin.RouterGroup) {
	group.GET("/tasks", r.handler.Task.ListTasks)
	group.POST("/tasks/:name/run", r.handler.Task.RunTask)
	group.POST("/tasks/:name/start", r.handler.Task.StartTask)
	group.POST("/tasks/:name/stop", r.handler.Task.StopTask)
	group.POST("/tasks/:name/reload", r.handler.Task.ReloadTask)
	group.PUT("/tasks/:name", r.handler.Task.UpdateTask)
	group.GET("/tasks/logs", r.handler.Task.ListLogs)
}
