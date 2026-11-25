// ipac_admin_api.go：IP 策略组/规则管理端点路由（——gw 治理页 IP tab）。
package adapters

import (
	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/modules/gw/api/keys"
)

// registerIPAdminRoutes 注册 IP 策略组/规则管理端点（权限码 admin.gw.ip-rules.*——
// 12570 种子同源；变更触发引擎 Reload+Bus 广播全节点）。
func registerIPAdminRoutes(authed gin.IRouter, deps Deps, secured securedChain) {
	g := authed.Group("/open/ip-groups")
	g.GET("", secured(keys.PermIPGroupsList, listIPGroupsHandler(deps))...)
	g.POST("", secured(keys.PermIPGroupsWrite, createIPGroupHandler(deps))...)
	g.PUT("/:id", secured(keys.PermIPGroupsWrite, updateIPGroupHandler(deps))...)
	g.DELETE("/:id", secured(keys.PermIPGroupsWrite, deleteIPGroupHandler(deps))...)
	r := authed.Group("/open/ip-rules")
	r.GET("", secured(keys.PermIPRulesList, listIPRulesHandler(deps))...)
	r.POST("", secured(keys.PermIPRulesWrite, createIPRuleHandler(deps))...)
	r.DELETE("/:id", secured(keys.PermIPRulesWrite, deleteIPRuleHandler(deps))...)
	authed.POST("/open/ip-reload", secured(keys.PermIPReload, reloadIPHandler(deps))...)
}
