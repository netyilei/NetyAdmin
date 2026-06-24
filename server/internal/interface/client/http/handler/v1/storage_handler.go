package v1

import (
	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	storageEntity "NetyAdmin/internal/domain/entity/storage"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"
	storageDto "NetyAdmin/internal/interface/admin/dto/storage"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	storageService "NetyAdmin/internal/service/storage"

	"github.com/gin-gonic/gin"
)

type ClientStorageHandler struct {
	recordSvc storageService.RecordService
}

func NewClientStorageHandler(recordSvc storageService.RecordService) *ClientStorageHandler {
	return &ClientStorageHandler{recordSvc: recordSvc}
}

func (h *ClientStorageHandler) GetUploadCredentials(c *gin.Context) {
	var req clientDto.GetClientCredentialsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	appObj, exists := c.Get("currentOpenApp")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized)
		return
	}
	app := appObj.(*openEntity.App)

	credReq := &storageDto.GetCredentialsReq{
		FileName:     req.FileName,
		ContentType:  req.ContentType,
		FileSize:     req.FileSize,
		BusinessType: req.BusinessType,
		BusinessID:   req.BusinessID,
	}

	result, err := h.recordSvc.GetUploadCredentials(c.Request.Context(), credReq, app.AppKey, storageEntity.UploadSourceClient, app.ID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, &clientDto.ClientCredentials{
		URL:         result.URL,
		Method:      result.Method,
		Headers:     result.Headers,
		ExpiresAt:   result.ExpiresAt,
		ObjectKey:   result.ObjectKey,
		Domain:      result.Domain,
		FinalURL:    result.FinalURL,
		ConfigID:    result.ConfigID,
		Region:      result.Region,
		Bucket:      result.Bucket,
		Endpoint:    result.Endpoint,
		PathPrefix:  result.PathPrefix,
		MaxFileSize: result.MaxFileSize,
		RecordID:    result.RecordID,
		Secret:      result.Secret,
	})
}

// CreateUploadRecord 上传成功通知：根据 recordId + secret 校验后将 pending 记录置为 uploaded。
func (h *ClientStorageHandler) CreateUploadRecord(c *gin.Context) {
	var req clientDto.CompleteClientUploadReq
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
