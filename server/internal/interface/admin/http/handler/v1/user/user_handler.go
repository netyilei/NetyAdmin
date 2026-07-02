package user

import (
	"github.com/gin-gonic/gin"

	userEntity "NetyAdmin/internal/domain/entity/user"
	userDto "NetyAdmin/internal/interface/admin/dto/user"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	userRepo "NetyAdmin/internal/repository/user"
	userSvc "NetyAdmin/internal/service/user"
)

type UserHandler struct {
	svc      userSvc.UserService
	cacheMgr cache.LazyCacheManager
}

func NewUserHandler(svc userSvc.UserService, cacheMgr cache.LazyCacheManager) *UserHandler {
	return &UserHandler{
		svc:      svc,
		cacheMgr: cacheMgr,
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

	query := &userRepo.UserRepoQuery{
		Username: req.Username,
		Nickname: req.Nickname,
		Gender:   req.Gender,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
	}

	users, total, err := h.svc.List(c.Request.Context(), req.Current, req.Size, query)
	if err != nil {
		response.Fail(c, err)
		return
	}

	type userWithLock struct {
		userEntity.User
		Locked bool `json:"locked"`
	}

	items := make([]userWithLock, 0, len(users))
	for _, u := range users {
		locked := false
		var lockVal string
		lockKey := cache.KeyLoginLock(u.ID)
		if err := h.cacheMgr.Get(c.Request.Context(), lockKey, &lockVal); err == nil && lockVal != "" {
			locked = true
		}
		items = append(items, userWithLock{User: u, Locked: locked})
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
// @Param        req body user.CreateUserReq true "创建用户请求"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req userDto.CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	u := &userEntity.User{
		Username: req.Username,
		Password: req.Password,
		Nickname: req.Nickname,
		Avatar:   req.Avatar,
		Gender:   req.Gender,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
	}

	if err := h.svc.Create(c.Request.Context(), u); err != nil {
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
// @Param        req body user.UpdateUserReq true "更新用户请求"
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

	u := &userEntity.User{
		ID:       id,
		Password: req.Password,
		Nickname: req.Nickname,
		Avatar:   req.Avatar,
		Gender:   req.Gender,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
	}

	if err := h.svc.Update(c.Request.Context(), u); err != nil {
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
// @Param        req body user.UpdateUserStatusReq true "更新状态请求"
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
	lockKey := cache.KeyLoginLock(id)
	_ = h.cacheMgr.Delete(c.Request.Context(), lockKey)
	retryKey := cache.KeyLoginRetryCount(id)
	_ = h.cacheMgr.Delete(c.Request.Context(), retryKey)
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
