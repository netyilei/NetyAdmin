package open_platform

import (
	"log"
	"strconv"

	"github.com/gin-gonic/gin"

	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	openRepo "NetyAdmin/internal/repository/open_platform"
	openSvc "NetyAdmin/internal/service/open_platform"
)

type OpenApiHandler struct {
	svc openSvc.OpenApiService
}

func NewOpenApiHandler(svc openSvc.OpenApiService) *OpenApiHandler {
	return &OpenApiHandler{svc: svc}
}

func (h *OpenApiHandler) List(c *gin.Context) {
	var req openDto.OpenApiQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	query := &openRepo.OpenApiRepoQuery{
		Page:     req.Current,
		PageSize: req.Size,
		Method:   req.Method,
		Path:     req.Path,
		Name:     req.Name,
		Group:    req.Group,
		Status:   req.Status,
	}

	list, total, err := h.svc.ListApis(c.Request.Context(), query)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

func (h *OpenApiHandler) Create(c *gin.Context) {
	var req openDto.CreateOpenApiReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	api := &openEntity.OpenApi{
		Method:      req.Method,
		Path:        req.Path,
		Name:        req.Name,
		Group:       req.Group,
		Description: req.Description,
		Status:      req.Status,
	}

	if err := h.svc.CreateApi(c.Request.Context(), api); err != nil {
		log.Printf("[OpenApi] Create error: %v", err)
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

func (h *OpenApiHandler) Update(c *gin.Context) {
	var req openDto.UpdateOpenApiReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	api := &openEntity.OpenApi{
		ID:          req.ID,
		Method:      req.Method,
		Path:        req.Path,
		Name:        req.Name,
		Group:       req.Group,
		Description: req.Description,
		Status:      req.Status,
	}

	if err := h.svc.UpdateApi(c.Request.Context(), api); err != nil {
		log.Printf("[OpenApi] Update error: %v", err)
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

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
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

func (h *OpenApiHandler) ListGrouped(c *gin.Context) {
	list, err := h.svc.ListGroupedApis(c.Request.Context())
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}
	response.Success(c, list)
}

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
		log.Printf("[OpenApi] GetScopeApis error: %v", err)
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}
	response.Success(c, list)
}

func (h *OpenApiHandler) UpdateScopeApis(c *gin.Context) {
	var req openDto.UpdateScopeApisReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.UpdateScopeApis(c.Request.Context(), req.ScopeID, req.ApiIDs); err != nil {
		log.Printf("[OpenApi] UpdateScopeApis error: %v", err)
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}
