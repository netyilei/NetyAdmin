package system

import (
	"github.com/gin-gonic/gin"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/configsync"
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

// bindConfigQuery 绑定配置查询参数，空 groupName 兜底为 cache_switches。
// 供 ListPublic / List handler 共用，避免重复代码（SRP）。
func bindConfigQuery(c *gin.Context) (systemDto.ConfigQuery, error) {
	var req systemDto.ConfigQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		return req, err
	}
	if req.GroupName == "" {
		req.GroupName = configsync.GroupCacheSwitches
	}
	return req, nil
}

// @Summary      获取配置分组（公开）
// @Description  根据组名获取多项配置（仅白名单分组，敏感字段脱敏）。供登录页等前置场景使用。
// @Tags         系统配置管理
// @Accept       json
// @Produce      json
// @Param        groupName query string true "配置组名（仅白名单内分组可访问）"
// @Success      200 {object} response.Response{data=[]NetyAdmin_internal_domain_vo_system.SysConfigVO} "配置列表"
// @Router       /admin/v1/system/configs/public [get]
func (h *ConfigHandler) ListPublic(c *gin.Context) {
	req, err := bindConfigQuery(c)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	configs, err := h.configSvc.ListByGroupPublic(c.Request.Context(), req.GroupName)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, configs)
}

// @Summary      获取配置分组（需权限）
// @Description  根据组名获取多项配置（敏感字段脱敏），需登录+权限。供管理后台设置页使用。
// @Tags         系统配置管理
// @Accept       json
// @Produce      json
// @Param        groupName query string true "配置组名"
// @Success      200 {object} response.Response{data=[]NetyAdmin_internal_domain_vo_system.SysConfigVO} "配置列表"
// @Router       /admin/v1/system/configs [get]
func (h *ConfigHandler) List(c *gin.Context) {
	req, err := bindConfigQuery(c)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	configs, err := h.configSvc.ListByGroup(c.Request.Context(), req.GroupName)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, configs)
}

// @Summary      更新/新增单个系统配置
// @Description  更新缓存开关或其他动态配置，修改后自动通过Redis广播全局重新加载内存字典。敏感字段若传 **** 则保留旧值。
// @Tags         系统配置管理
// @Accept       json
// @Produce      json
// @Param        req body systemDto.UpdateConfigReq true "配置信息"
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
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      测试邮件发送
// @Description  使用当前邮件配置发送测试邮件，验证配置是否正确
// @Tags         系统配置管理
// @Accept       json
// @Produce      json
// @Param        req body systemDto.TestEmailReq true "测试邮件信息"
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

	// 收敛 Handler 跨层调用（spec B10）：邮件发送下沉到 service，handler 不再直接调 emailDriver
	if err := h.configSvc.TestEmail(c.Request.Context(), req.Receiver); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
