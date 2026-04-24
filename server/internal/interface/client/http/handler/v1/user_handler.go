package v1

import (
	"strings"

	"github.com/gin-gonic/gin"

	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	storageEntity "NetyAdmin/internal/domain/entity/storage"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	storageService "NetyAdmin/internal/service/storage"
	userSvcPkg "NetyAdmin/internal/service/user"
)

type UserHandler struct {
	userSvc   userSvcPkg.UserService
	recordSvc storageService.RecordService
}

func NewUserHandler(userSvc userSvcPkg.UserService, recordSvc storageService.RecordService) *UserHandler {
	return &UserHandler{
		userSvc:   userSvc,
		recordSvc: recordSvc,
	}
}

// Register 注册接口
func (h *UserHandler) Register(c *gin.Context) {
	var req clientDto.UserRegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数校验失败")
		return
	}

	uid, err := h.userSvc.Register(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, gin.H{"id": uid})
}

// Login 登录接口
func (h *UserHandler) Login(c *gin.Context) {
	var req clientDto.UserLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数校验失败")
		return
	}

	// 记录登录 IP
	loginVO, err := h.userSvc.Login(c.Request.Context(), &req, c.ClientIP())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, loginVO)
}

// GetProfile 获取个人资料
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized)
		return
	}

	info, err := h.userSvc.GetInfo(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, info)
}

// UpdateProfile 更新个人资料
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString("userID")
	var req clientDto.UserUpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.userSvc.UpdateProfile(c.Request.Context(), userID, &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// ResetPassword 找回密码
func (h *UserHandler) ResetPassword(c *gin.Context) {
	var req clientDto.UserResetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.userSvc.ResetPassword(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// ChangePassword 修改密码
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("userID")
	var req clientDto.UserChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.userSvc.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// DeleteAccount 注销账号
func (h *UserHandler) DeleteAccount(c *gin.Context) {
	userID := c.GetString("userID")
	if err := h.userSvc.DeleteAccount(c.Request.Context(), userID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// GetUploadToken 获取上传凭证
func (h *UserHandler) GetUploadToken(c *gin.Context) {
	userID := c.GetString("userID")

	var storageID uint
	if appObj, exists := c.Get("currentOpenApp"); exists {
		app := appObj.(*openEntity.App)
		storageID = app.StorageID
	}

	token, err := h.userSvc.GetUploadToken(c.Request.Context(), userID, storageID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, token)
}

// RecordUpload 记录用户上传结果
func (h *UserHandler) RecordUpload(c *gin.Context) {
	userID := c.GetString("userID")
	var req clientDto.CreateUserUploadRecordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	appObj, exists := c.Get("currentOpenApp")
	var appKey string
	if exists {
		app := appObj.(*openEntity.App)
		appKey = app.AppKey
	}

	err := h.recordSvc.RecordUpload(
		c.Request.Context(),
		req.StorageConfigID,
		req.FileName,
		req.FileName,
		req.ObjectKey,
		"",
		req.FileSize,
		req.MimeType,
		req.MD5,
		storageEntity.UploadSourceUser,
		userID,
		nil,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
		req.BusinessType,
		req.BusinessID,
		appKey,
	)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// Logout 退出登录
func (h *UserHandler) Logout(c *gin.Context) {
	userID := c.GetString("userID")
	authHeader := c.GetHeader("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if err := h.userSvc.Logout(c.Request.Context(), userID, token); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// RefreshToken 刷新令牌
func (h *UserHandler) RefreshToken(c *gin.Context) {
	refreshToken := c.Query("refreshToken")
	if refreshToken == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "缺少刷新令牌")
		return
	}

	loginVO, err := h.userSvc.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, loginVO)
}
