package system

import (
	"github.com/gin-gonic/gin"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	systemService "NetyAdmin/internal/service/system"
)

type ConfigHandler struct {
	configSvc systemService.ConfigService
}

func NewConfigHandler(configSvc systemService.ConfigService) *ConfigHandler {
	return &ConfigHandler{
		configSvc: configSvc,
	}
}

// @Summary      获取配置分组
// @Description  根据组名获取多项配置，例如 cache_switches
// @Tags         系统配置管理
// @Accept       json
// @Produce      json
// @Param        groupName query string true "配置组名"
// @Success      200 {object} response.Response{data=[]system.SysConfigVO} "配置列表"
// @Router       /admin/v1/system/configs [get]
func (h *ConfigHandler) ListByGroup(c *gin.Context) {
	var req systemDto.ConfigQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if req.GroupName == "" {
		req.GroupName = "cache_switches" // default for now
	}

	configs, err := h.configSvc.ListByGroup(c.Request.Context(), req.GroupName)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, "获取配置失败")
		return
	}

	response.Success(c, configs)
}

// @Summary      更新/新增单个系统配置
// @Description  更新缓存开关或其他动态配置，修改后自动通过Redis广播全局重新加载内存字典
// @Tags         系统配置管理
// @Accept       json
// @Produce      json
// @Param        req body system.UpdateConfigReq true "配置信息"
// @Success      200 {object} response.Response "保存成功"
// @Router       /admin/v1/system/configs [put]
func (h *ConfigHandler) Upsert(c *gin.Context) {
	var req systemDto.UpdateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数格式错误")
		return
	}

	operatorID := c.GetUint("userID")

	if err := h.configSvc.Upsert(c.Request.Context(), &req, operatorID); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, "更新配置失败")
		return
	}

	response.Success(c, nil)
}
