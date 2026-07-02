package v1

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	storageEntity "NetyAdmin/internal/domain/entity/storage"
	userVO "NetyAdmin/internal/domain/vo/user"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"
	storageDto "NetyAdmin/internal/interface/admin/dto/storage"
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
// @Summary      用户注册
// @Description  客户端用户注册账号
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Param        req body clientDto.UserRegisterReq true "注册请求"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/register [post]
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
// @Summary      用户登录
// @Description  客户端用户登录并获取访问令牌
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Param        req body clientDto.UserLoginReq true "登录请求"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/login [post]
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
// @Summary      获取个人资料
// @Description  获取当前登录用户的个人资料信息
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/profile [get]
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
// @Summary      更新个人资料
// @Description  更新当前登录用户的个人资料信息
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Param        req body clientDto.UserUpdateProfileReq true "更新个人资料请求"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/profile [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}
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
// @Summary      重置密码
// @Description  通过验证码找回并重置用户密码
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Param        req body clientDto.UserResetPasswordReq true "重置密码请求"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/reset-password [post]
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
// @Summary      修改密码
// @Description  当前登录用户修改登录密码
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Param        req body clientDto.UserChangePasswordReq true "修改密码请求"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/password [put]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}
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
// @Summary      注销账号
// @Description  当前登录用户注销账号
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/account [delete]
func (h *UserHandler) DeleteAccount(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}
	if err := h.userSvc.DeleteAccount(c.Request.Context(), userID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// GetUploadToken 获取上传凭证：签发 presigned URL 并落 pending 记录，返回 recordId + secret。
// fileName 建议传入；未传时用时间戳兜底以保证 objectKey 合法。
// @Summary      获取上传凭证
// @Description  签发预签名上传URL并创建待处理记录，返回recordId与secret
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Param        fileName query string false "文件名"
// @Param        contentType query string false "内容类型"
// @Param        businessType query string false "业务类型"
// @Param        businessId query string false "业务ID"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/upload-token [get]
func (h *UserHandler) GetUploadToken(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	fileName := c.Query("fileName")
	if fileName == "" {
		fileName = c.Query("filename")
	}
	if fileName == "" {
		fileName = fmt.Sprintf("upload-%d.bin", time.Now().UnixNano())
	}

	var appKey string
	var configID uint
	if appObj, exists := c.Get("currentOpenApp"); exists {
		app, ok := appObj.(*openEntity.App)
		if ok {
			appKey = app.AppKey
			configID = app.StorageID
		}
	}

	credReq := &storageDto.GetCredentialsReq{
		ConfigID:     configID,
		FileName:     fileName,
		ContentType:  c.Query("contentType"),
		BusinessType: c.Query("businessType"),
		BusinessID:   c.Query("businessId"),
	}

	cred, err := h.recordSvc.GetUploadCredentials(c.Request.Context(), credReq, appKey, storageEntity.UploadSourceUser, userID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, &userVO.UploadTokenVO{
		UploadURL:       cred.URL,
		StorageConfigID: cred.ConfigID,
		ObjectKey:       cred.ObjectKey,
		FinalURL:        cred.FinalURL,
		RecordID:        cred.RecordID,
		Secret:          cred.Secret,
	})
}

// RecordUpload 上传成功通知：根据 recordId + secret 校验后将 pending 记录置为 uploaded。
// @Summary      记录上传结果
// @Description  上传成功后通知服务端，根据recordId与secret校验后将记录置为已上传
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Param        req body clientDto.CreateUserUploadRecordReq true "记录上传请求"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/upload-record [post]
func (h *UserHandler) RecordUpload(c *gin.Context) {
	var req clientDto.CreateUserUploadRecordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	result, err := h.recordSvc.CompleteUpload(
		c.Request.Context(),
		req.RecordID,
		req.Secret,
		req.ObjectKey,
		req.FileURL,
		req.FileSize,
		req.MimeType,
		req.MD5,
	)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

// Logout 退出登录
// @Summary      退出登录
// @Description  当前登录用户退出登录并失效令牌
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}
	authHeader := c.GetHeader("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if err := h.userSvc.Logout(c.Request.Context(), userID, token); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// RefreshToken 刷新令牌
// @Summary      刷新令牌
// @Description  使用刷新令牌获取新的访问令牌
// @Tags         客户端-用户
// @Accept       json
// @Produce      json
// @Param        refreshToken query string true "刷新令牌"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/user/refresh-token [post]
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
