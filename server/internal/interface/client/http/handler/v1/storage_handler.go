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

	result, err := h.recordSvc.GetUploadCredentials(c.Request.Context(), credReq, app.AppKey)
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
	})
}

func (h *ClientStorageHandler) CreateUploadRecord(c *gin.Context) {
	var req clientDto.CreateClientRecordReq
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

	err := h.recordSvc.RecordUpload(
		c.Request.Context(),
		0,
		req.FileName,
		req.FileName,
		req.ObjectKey,
		"",
		req.FileSize,
		req.MimeType,
		req.MD5,
		storageEntity.UploadSourceClient,
		app.ID,
		nil,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
		req.BusinessType,
		req.BusinessID,
		app.AppKey,
	)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
