package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/domain/entity"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/middleware"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	systemService "NetyAdmin/internal/service/system"
)

type AdminHandler struct {
	adminService systemService.AdminService
}

func NewAdminHandler(adminService systemService.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

// @Summary      获取管理员列表
// @Description  分页查询管理员列表，支持按用户名、昵称、手机号、邮箱、状态、性别筛选
// @Tags         管理员管理
// @Accept       json
// @Produce      json
// @Param        username query string false "用户名"
// @Param        nickname query string false "昵称"
// @Param        phone query string false "手机号"
// @Param        email query string false "邮箱"
// @Param        status query string false "状态"
// @Param        gender query string false "性别"
// @Param        current query int false "当前页"
// @Param        size query int false "每页数量"
// @Success      200 {object} response.Response "管理员分页列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/admins [get]
func (h *AdminHandler) List(c *gin.Context) {
	var req systemDto.AdminQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if req.Current <= 0 {
		req.Current = 1
	}
	if req.Size <= 0 {
		req.Size = entity.DefaultPageSize
	}

	admins, total, err := h.adminService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, admins)
}

// @Summary      创建管理员
// @Description  创建新的管理员账号，设置用户名、密码、昵称、联系方式、状态及角色
// @Tags         管理员管理
// @Accept       json
// @Produce      json
// @Param        req body system.CreateAdminReq true "管理员创建参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/admins [post]
func (h *AdminHandler) Create(c *gin.Context) {
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}
	operatorIsSuper := middleware.IsSuperAdminFromContext(c)

	var req systemDto.CreateAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.adminService.Create(c.Request.Context(), &req, operatorID, operatorIsSuper)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "管理员创建成功", gin.H{"id": id})
}

// @Summary      更新管理员
// @Description  根据管理员ID更新管理员账号信息，包括资料、密码及角色
// @Tags         管理员管理
// @Accept       json
// @Produce      json
// @Param        id path int true "管理员ID"
// @Param        req body system.UpdateAdminReq true "管理员更新参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/admins/{id} [put]
func (h *AdminHandler) Update(c *gin.Context) {
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}
	operatorIsSuper := middleware.IsSuperAdminFromContext(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的管理员ID")
		return
	}

	var req systemDto.UpdateAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}
	req.ID = uint(id)

	if err := h.adminService.Update(c.Request.Context(), &req, operatorID, operatorIsSuper); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "管理员更新成功", nil)
}

// @Summary      删除管理员
// @Description  根据管理员ID删除单个管理员账号
// @Tags         管理员管理
// @Accept       json
// @Produce      json
// @Param        id path int true "管理员ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/admins/{id} [delete]
func (h *AdminHandler) Delete(c *gin.Context) {
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的管理员ID")
		return
	}

	if uint(id) == operatorID {
		response.FailWithCode(c, errorx.CodeBadRequest, "不能删除自己的账号")
		return
	}

	if err := h.adminService.Delete(c.Request.Context(), uint(id), operatorID); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "管理员删除成功", nil)
}

// @Summary      批量删除管理员
// @Description  根据管理员ID列表批量删除管理员账号
// @Tags         管理员管理
// @Accept       json
// @Produce      json
// @Param        ids body []uint true "待删除的管理员ID列表"
// @Success      200 {object} response.Response "批量删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/admins/batch [delete]
func (h *AdminHandler) DeleteBatch(c *gin.Context) {
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req struct {
		Ids []uint `form:"ids" json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	for _, id := range req.Ids {
		if id == operatorID {
			response.FailWithCode(c, errorx.CodeBadRequest, "不能删除自己的账号")
			return
		}
	}

	if err := h.adminService.DeleteBatch(c.Request.Context(), req.Ids, operatorID); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "管理员批量删除成功", nil)
}

// @Summary      获取管理员详情
// @Description  根据管理员ID获取管理员详细信息
// @Tags         管理员管理
// @Accept       json
// @Produce      json
// @Param        id path int true "管理员ID"
// @Success      200 {object} response.Response "管理员详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/admins/{id} [get]
func (h *AdminHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的管理员ID")
		return
	}

	admin, err := h.adminService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, admin)
}
