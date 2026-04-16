package open_platform

import (
	"strconv"

	"github.com/gin-gonic/gin"

	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	openRepo "NetyAdmin/internal/repository/open_platform"
	openSvc "NetyAdmin/internal/service/open_platform"
)

type AppHandler struct {
	svc openSvc.AppService
}

func NewAppHandler(svc openSvc.AppService) *AppHandler {
	return &AppHandler{svc: svc}
}

// List 获取应用列表
func (h *AppHandler) List(c *gin.Context) {
	var req openDto.AppQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	query := &openRepo.AppRepoQuery{
		Page:     req.Current,
		PageSize: req.Size,
		Name:     req.Name,
		AppKey:   req.AppKey,
		Type:     req.Type,
		Status:   req.Status,
	}

	list, total, err := h.svc.ListApps(c.Request.Context(), query)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// Create 新增应用
func (h *AppHandler) Create(c *gin.Context) {
	var req openDto.CreateAppReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	app := &openEntity.App{
		Name:       req.Name,
		Type:       req.Type,
		Status:     req.Status,
		IPStrategy: req.IPStrategy,
		Remark:     req.Remark,
	}

	if err := h.svc.CreateApp(c.Request.Context(), app, req.Scopes); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

// Update 修改应用
func (h *AppHandler) Update(c *gin.Context) {
	var req openDto.UpdateAppReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	app := &openEntity.App{
		ID:         req.ID,
		Name:       req.Name,
		Type:       req.Type,
		Status:     req.Status,
		IPStrategy: req.IPStrategy,
		Remark:     req.Remark,
	}

	if err := h.svc.UpdateApp(c.Request.Context(), app, req.Scopes); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

// Delete 删除应用
func (h *AppHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.DeleteApp(c.Request.Context(), id); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

// ResetSecret 重置密钥
func (h *AppHandler) ResetSecret(c *gin.Context) {
	var req openDto.ResetSecretReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	newSecret, err := h.svc.ResetAppSecret(c.Request.Context(), req.ID)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, gin.H{"appSecret": newSecret})
}

// GetScopes 获取应用的权限范围
func (h *AppHandler) GetScopes(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	scopes, err := h.svc.GetAppScopes(c.Request.Context(), id)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, scopes)
}

// ListAvailableScopes 获取所有可用的权限范围
func (h *AppHandler) ListAvailableScopes(c *gin.Context) {
	scopes, err := h.svc.ListAvailableScopes(c.Request.Context())
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, scopes)
}

// ListScopeGroups 获取所有权限分组
func (h *AppHandler) ListScopeGroups(c *gin.Context) {
	list, err := h.svc.ListScopeGroups(c.Request.Context())
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}
	response.Success(c, list)
}

// CreateScopeGroup 新增权限分组
func (h *AppHandler) CreateScopeGroup(c *gin.Context) {
	var req openEntity.AppScopeGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.CreateScopeGroup(c.Request.Context(), &req); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}
	response.Success(c, nil)
}

// UpdateScopeGroup 修改权限分组
func (h *AppHandler) UpdateScopeGroup(c *gin.Context) {
	var req openEntity.AppScopeGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.UpdateScopeGroup(c.Request.Context(), &req); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
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
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}
