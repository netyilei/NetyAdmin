package system

import (
	"github.com/gin-gonic/gin"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/errorx"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/response"
	systemService "NetyAdmin/internal/service/system"
)

type ConfigHandler struct {
	configSvc  systemService.ConfigService
	emailDriver msgPkg.Driver
}

func NewConfigHandler(configSvc systemService.ConfigService, emailDriver msgPkg.Driver) *ConfigHandler {
	return &ConfigHandler{
		configSvc:  configSvc,
		emailDriver: emailDriver,
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
		req.GroupName = "cache_switches"
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

	operatorID := c.GetUint("adminID")

	if err := h.configSvc.Upsert(c.Request.Context(), &req, operatorID); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, "更新配置失败")
		return
	}

	response.Success(c, nil)
}

// @Summary      测试邮件发送
// @Description  使用当前邮件配置发送测试邮件，验证配置是否正确
// @Tags         系统配置管理
// @Accept       json
// @Produce      json
// @Param        req body system.TestEmailReq true "测试邮件信息"
// @Success      200 {object} response.Response "发送成功"
// @Router       /admin/v1/system/test-email [post]
func (h *ConfigHandler) TestEmail(c *gin.Context) {
	var req systemDto.TestEmailReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数格式错误")
		return
	}

	if req.Receiver == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "收件人地址不能为空")
		return
	}

	if h.emailDriver == nil {
		response.FailWithCode(c, errorx.CodeInternalError, "邮件驱动未初始化")
		return
	}

	err := h.emailDriver.Send(c.Request.Context(), req.Receiver, "NetyAdmin 测试邮件", "<h2>测试邮件</h2><p>这是一封来自 NetyAdmin 的测试邮件，如果您收到了此邮件，说明邮件配置正确。</p>", nil)
	if err != nil {
		response.FailWithCode(c, errorx.CodeEmailTestFailed, err.Error())
		return
	}

	response.Success(c, nil)
}
