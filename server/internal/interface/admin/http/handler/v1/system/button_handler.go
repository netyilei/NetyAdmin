package system

import (
	"strconv"

	"github.com/gin-gonic/gin"

	systemDto "silentorder/internal/interface/admin/dto/system"
	"silentorder/internal/pkg/errorx"
	"silentorder/internal/pkg/response"
)

func (h *SystemHandler) GetAdminButtonList(c *gin.Context) {
	var req systemDto.ButtonQuery
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

	buttons, total, err := h.buttonService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, buttons)
}

func (h *SystemHandler) AddAdminButton(c *gin.Context) {
	var req systemDto.CreateButtonReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.buttonService.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "按钮创建成功", gin.H{"id": id})
}

func (h *SystemHandler) UpdateAdminButton(c *gin.Context) {
	var req systemDto.UpdateButtonReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.buttonService.Update(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "按钮更新成功", nil)
}

func (h *SystemHandler) DeleteAdminButton(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的按钮ID")
		return
	}

	if err := h.buttonService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "按钮删除成功", nil)
}

func (h *SystemHandler) GetAllAdminButton(c *gin.Context) {
	buttons, err := h.buttonService.GetAll(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, buttons)
}

func (h *SystemHandler) GetAdminButtonByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的按钮ID")
		return
	}

	button, err := h.buttonService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, button)
}
