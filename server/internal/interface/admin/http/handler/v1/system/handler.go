package system

import (
	systemService "netyadmin/internal/service/system"
)

type SystemHandler struct {
	roleService   systemService.RoleService
	menuService   systemService.MenuService
	apiService    systemService.APIService
	buttonService systemService.ButtonService
	adminService  systemService.AdminService

	Config *ConfigHandler
	Task   *TaskHandler
	Dict   *DictHandler
}

func NewSystemHandler(
	roleService systemService.RoleService,
	menuService systemService.MenuService,
	apiService systemService.APIService,
	buttonService systemService.ButtonService,
	adminService systemService.AdminService,
	configSvc systemService.ConfigService,
	taskSvc systemService.TaskService,
	dictSvc systemService.DictService,
) *SystemHandler {
	return &SystemHandler{
		roleService:   roleService,
		menuService:   menuService,
		apiService:    apiService,
		buttonService: buttonService,
		adminService:  adminService,
		Config:        NewConfigHandler(configSvc),
		Task:          NewTaskHandler(taskSvc),
		Dict:          NewDictHandler(dictSvc),
	}
}
