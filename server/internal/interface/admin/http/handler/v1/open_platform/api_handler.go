package open_platform

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"

	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/response"
	openSvc "NetyAdmin/internal/service/open_platform"
)

type OpenApiHandler struct {
	svc openSvc.OpenApiService
}

func NewOpenApiHandler(svc openSvc.OpenApiService) *OpenApiHandler {
	return &OpenApiHandler{svc: svc}
}

// @Summary      获取开放API列表
// @Description  分页获取开放平台API列表，支持按方法、路径、名称、分组、状态筛选
// @Tags         开放API管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        method query string false "请求方法"
// @Param        path query string false "API路径"
// @Param        name query string false "API名称"
// @Param        group query string false "API分组"
// @Param        status query int false "状态(0:禁用 1:启用)"
// @Success      200 {object} response.Response "API列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apis [get]
func (h *OpenApiHandler) List(c *gin.Context) {
	var req openDto.OpenApiQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	req.Current, req.Size = pagination.NormalizePagination(req.Current, req.Size)

	// 收敛 Handler 跨层调用（spec B10）：service 接收 admin DTO，不再依赖 handler 构造 repo query
	list, total, err := h.svc.ListApis(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// @Summary      新增开放API
// @Description  创建开放平台API
// @Tags         开放API管理
// @Accept       json
// @Produce      json
// @Param        req body open_platform.CreateOpenApiReq true "创建API请求"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apis [post]
func (h *OpenApiHandler) Create(c *gin.Context) {
	var req openDto.CreateOpenApiReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.CreateApi(c.Request.Context(), &req); err != nil {
		slog.Error("[OpenApi] Create error", "err", err)
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      修改开放API
// @Description  更新开放平台API信息
// @Tags         开放API管理
// @Accept       json
// @Produce      json
// @Param        req body open_platform.UpdateOpenApiReq true "更新API请求"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apis [put]
func (h *OpenApiHandler) Update(c *gin.Context) {
	var req openDto.UpdateOpenApiReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.UpdateApi(c.Request.Context(), &req); err != nil {
		slog.Error("[OpenApi] Update error", "err", err)
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      删除开放API
// @Description  根据ID删除开放平台API
// @Tags         开放API管理
// @Accept       json
// @Produce      json
// @Param        id path int true "API ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apis/{id} [delete]
func (h *OpenApiHandler) Delete(c *gin.Context) {
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

	if err := h.svc.DeleteApi(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      获取分组API列表
// @Description  按分组获取开放平台API列表
// @Tags         开放API管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "分组API列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apis/grouped [get]
func (h *OpenApiHandler) ListGrouped(c *gin.Context) {
	list, err := h.svc.ListGroupedApis(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

// @Summary      获取权限范围的API列表
// @Description  根据权限范围ID获取其关联的API列表
// @Tags         开放API管理
// @Accept       json
// @Produce      json
// @Param        scopeId query int true "权限范围ID"
// @Success      200 {object} response.Response "API列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apis/scope-apis [get]
func (h *OpenApiHandler) GetScopeApis(c *gin.Context) {
	idStr := c.Query("scopeId")
	if idStr == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	list, err := h.svc.GetScopeApis(c.Request.Context(), id)
	if err != nil {
		slog.Error("[OpenApi] GetScopeApis error", "err", err)
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

// @Summary      更新权限范围的API
// @Description  更新指定权限范围所关联的API列表
// @Tags         开放API管理
// @Accept       json
// @Produce      json
// @Param        req body open_platform.UpdateScopeApisReq true "更新权限范围API请求"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open/apis/scope-apis [put]
func (h *OpenApiHandler) UpdateScopeApis(c *gin.Context) {
	var req openDto.UpdateScopeApisReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.UpdateScopeApis(c.Request.Context(), req.ScopeID, req.ApiIDs); err != nil {
		slog.Error("[OpenApi] UpdateScopeApis error", "err", err)
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
