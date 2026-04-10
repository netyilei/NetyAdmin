package system

import (
	"strconv"

	"github.com/gin-gonic/gin"

	systemDto "silentorder/internal/interface/admin/dto/system"
	"silentorder/internal/pkg/errorx"
	"silentorder/internal/pkg/response"
)

func (h *SystemHandler) GetAdminAPIList(c *gin.Context) {
	var req systemDto.APIQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if req.Current <= 0 {
		req.Current = 1
	}
	if req.Size <= 0 {
		req.Size = 10
	}

	apis, total, err := h.apiService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, apis)
}

func (h *SystemHandler) AddAdminAPI(c *gin.Context) {
	var req systemDto.CreateAPIReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.apiService.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "API创建成功", gin.H{"id": id})
}

func (h *SystemHandler) UpdateAdminAPI(c *gin.Context) {
	var req systemDto.UpdateAPIReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.apiService.Update(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "API更新成功", nil)
}

func (h *SystemHandler) DeleteAdminAPI(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的API ID")
		return
	}

	if err := h.apiService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "API删除成功", nil)
}

func (h *SystemHandler) GetAdminAPIByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的API ID")
		return
	}

	api, err := h.apiService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, api)
}

func (h *SystemHandler) GetAllAdminAPI(c *gin.Context) {
	apis, err := h.apiService.GetAll(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, apis)
}
