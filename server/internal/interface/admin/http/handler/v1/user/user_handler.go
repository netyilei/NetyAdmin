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

func (h *UserHandler) Unlock(c *gin.Context) {
	id := c.Param("id")
	lockKey := cache.KeyLoginLock(id)
	_ = h.cacheMgr.Delete(c.Request.Context(), lockKey)
	retryKey := cache.KeyLoginRetryCount(id)
	_ = h.cacheMgr.Delete(c.Request.Context(), retryKey)
	response.Success(c, nil)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
