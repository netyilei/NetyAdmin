package open_platform

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/response"
	openSvc "NetyAdmin/internal/service/open_platform"
)

type AppHandler struct {
	svc openSvc.AppService
}

func NewAppHandler(svc openSvc.AppService) *AppHandler {
	return &AppHandler{svc: svc}
}

// @Summary      获取应用列表
// @Description  分页获取开放平台应用列表，支持按名称、AppKey、状态筛选
// @Tags         应用管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        name query string false "应用名称"
// @Param        appKey query string false "应用AppKey"
// @Param        status query int false "状态(0:禁用 1:启用)"
// @Success      200 {object} response.Response "应用列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apps [get]
func (h *AppHandler) List(c *gin.Context) {
	var req openDto.AppQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	req.Current, req.Size = pagination.NormalizePagination(req.Current, req.Size)

	// 收敛 Handler 跨层调用（spec B10）：service 接收 admin DTO，不再依赖 handler 构造 repo query
	list, total, err := h.svc.ListApps(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// @Summary      新增应用
// @Description  创建开放平台应用，并关联权限范围
// @Tags         应用管理
// @Accept       json
// @Produce      json
// @Param        req body open_platform.CreateAppReq true "创建应用请求"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apps [post]
func (h *AppHandler) Create(c *gin.Context) {
	var req openDto.CreateAppReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.CreateApp(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      修改应用
// @Description  更新开放平台应用信息及其权限范围
// @Tags         应用管理
// @Accept       json
// @Produce      json
// @Param        req body open_platform.UpdateAppReq true "更新应用请求"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apps [put]
func (h *AppHandler) Update(c *gin.Context) {
	var req openDto.UpdateAppReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.UpdateApp(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      删除应用
// @Description  根据应用ID删除开放平台应用
// @Tags         应用管理
// @Accept       json
// @Produce      json
// @Param        id path string true "应用ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apps/{id} [delete]
func (h *AppHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.DeleteApp(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      重置密钥
// @Description  重置开放平台应用的AppSecret，返回新的密钥
// @Tags         应用管理
// @Accept       json
// @Produce      json
// @Param        req body open_platform.ResetSecretReq true "重置密钥请求"
// @Success      200 {object} response.Response "新密钥"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apps/reset-secret [put]
func (h *AppHandler) ResetSecret(c *gin.Context) {
	var req openDto.ResetSecretReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	newSecret, err := h.svc.ResetAppSecret(c.Request.Context(), req.ID)
	if err != nil {
		var bizErr *errorx.BizError
		if errors.As(err, &bizErr) && bizErr.Code == errorx.CodeNotFound {
			response.FailWithCode(c, errorx.CodeNotFound)
			return
		}
		response.Fail(c, err)
		return
	}

	response.Success(c, gin.H{"appSecret": newSecret})
}

// @Summary      关联IP规则
// @Description  将IP规则关联到指定应用
// @Tags         应用管理
// @Accept       json
// @Produce      json
// @Param        req body open_platform.LinkIPRulesReq true "关联IP规则请求"
// @Success      200 {object} response.Response "关联成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apps/ip-rules [put]
func (h *AppHandler) LinkIPRules(c *gin.Context) {
	var req openDto.LinkIPRulesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.LinkIPRules(c.Request.Context(), req.ID, req.RuleIDs); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      获取应用权限范围
// @Description  根据应用ID获取应用已授权的权限范围列表
// @Tags         应用管理
// @Accept       json
// @Produce      json
// @Param        id query string true "应用ID"
// @Success      200 {object} response.Response "权限范围列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apps/scopes [get]
func (h *AppHandler) GetScopes(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	scopes, err := h.svc.GetAppScopes(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, scopes)
}

// @Summary      获取可用权限范围
// @Description  获取所有可用的权限范围列表
// @Tags         应用管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "可用权限范围列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apps/available-scopes [get]
func (h *AppHandler) ListAvailableScopes(c *gin.Context) {
	scopes, err := h.svc.ListAvailableScopes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, scopes)
}

// ListScopeGroups 获取所有权限分组
func (h *AppHandler) ListScopeGroups(c *gin.Context) {
	list, err := h.svc.ListScopeGroups(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

// CreateScopeGroup 新增权限分组
func (h *AppHandler) CreateScopeGroup(c *gin.Context) {
	var req openDto.CreateScopeGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.CreateScopeGroup(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// UpdateScopeGroup 修改权限分组
func (h *AppHandler) UpdateScopeGroup(c *gin.Context) {
	var req openDto.UpdateScopeGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.UpdateScopeGroup(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// DeleteScopeGroup 删除权限分组
func (h *AppHandler) DeleteScopeGroup(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.DeleteScopeGroup(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
