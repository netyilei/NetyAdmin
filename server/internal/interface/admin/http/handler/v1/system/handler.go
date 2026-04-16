package system

import (
	systemService "NetyAdmin/internal/service/system"
)

type SystemHandler struct {
	roleService   systemService.RoleService
	menuService   systemService.MenuService
	apiService    systemService.APIService
	buttonService systemService.ButtonService
	adminService  systemService.AdminService

	Config *ConfigHandler
}

func NewSystemHandler(
	roleService systemService.RoleService,
	menuService systemService.MenuService,
	apiService systemService.APIService,
	buttonService systemService.ButtonService,
	adminService systemService.AdminService,
	configSvc systemService.ConfigService,
) *SystemHandler {
	return &SystemHandler{
		roleService:   roleService,
		menuService:   menuService,
		apiService:    apiService,
		buttonService: buttonService,
		adminService:  adminService,
		Config:        NewConfigHandler(configSvc),
	}
}
