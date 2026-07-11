package user

import (
	"github.com/gin-gonic/gin"

	userDto "NetyAdmin/internal/interface/admin/dto/user"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/response"
	userSvc "NetyAdmin/internal/service/user"
)

type UserHandler struct {
	svc userSvc.UserAdminService
}

func NewUserHandler(svc userSvc.UserAdminService) *UserHandler {
	return &UserHandler{
		svc: svc,
	}
}

// @Summary      获取用户列表
// @Description  分页获取终端用户列表，支持按用户名、昵称、性别、手机号、邮箱、状态筛选
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        username query string false "用户名"
// @Param        nickname query string false "昵称"
// @Param        gender query string false "性别(0:未知 1:男 2:女)"
// @Param        phone query string false "手机号"
// @Param        email query string false "邮箱"
// @Param        status query string false "状态(0:禁用 1:正常)"
// @Success      200 {object} response.Response "用户列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/users [get]
func (h *UserHandler) List(c *gin.Context) {
	var req userDto.UserQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	req.Current, req.Size = pagination.NormalizePagination(req.Current, req.Size)

	// 收敛 Handler 跨层调用（spec B10）：locked 字段由 service 内部查询 cacheSlow 填充，
	// handler 不再直接操作 cacheSlow，直接消费 service 返回的 []UserWithLock
	items, total, err := h.svc.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, items)
}

// @Summary      用户自动补全
// @Description  根据关键字搜索用户，用于输入框自动补全
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        keyword query string false "搜索关键字"
// @Success      200 {object} response.Response "用户列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/users/autocomplete [get]
func (h *UserHandler) Autocomplete(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		response.Success(c, []any{})
		return
	}
	users, err := h.svc.SearchForAutocomplete(c.Request.Context(), keyword, 20)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, users)
}

// @Summary      新增用户
// @Description  创建终端用户
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        req body userDto.CreateUserReq true "创建用户请求"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req userDto.CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	// entity 构造下沉到 service 层（spec D4：handler 不再构造 entity）
	if err := h.svc.Create(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      修改用户
// @Description  根据用户ID更新用户信息
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        id path string true "用户ID"
// @Param        req body userDto.UpdateUserReq true "更新用户请求"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req userDto.UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	// entity 构造下沉到 service 层（spec D4：handler 不再构造 entity）
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      修改用户状态
// @Description  根据用户ID更新用户状态
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        id path string true "用户ID"
// @Param        req body userDto.UpdateUserStatusReq true "更新状态请求"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/users/{id}/status [patch]
func (h *UserHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var req userDto.UpdateUserStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      解锁用户
// @Description  根据用户ID解除登录锁定状态
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        id path string true "用户ID"
// @Success      200 {object} response.Response "解锁成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/users/{id}/unlock [post]
func (h *UserHandler) Unlock(c *gin.Context) {
	id := c.Param("id")
	// 收敛 Handler 跨层调用（spec B10）：解锁逻辑下沉到 service，handler 不再直接操作 cacheSlow
	if err := h.svc.UnlockUser(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      删除用户
// @Description  根据用户ID删除用户
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        id path string true "用户ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
