package storage

import (
	"strconv"

	"github.com/gin-gonic/gin"

	storageDto "NetyAdmin/internal/interface/admin/dto/storage"
	storageEntity "NetyAdmin/internal/domain/entity/storage"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	storageService "NetyAdmin/internal/service/storage"
)

type StorageHandler struct {
	configService storageService.ConfigService
	recordService storageService.RecordService
}

func NewStorageHandler(
	configService storageService.ConfigService,
	recordService storageService.RecordService,
) *StorageHandler {
	return &StorageHandler{
		configService: configService,
		recordService: recordService,
	}
}

func (h *StorageHandler) GetStorageConfigList(c *gin.Context) {
	var req storageDto.ConfigQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	configs, total, err := h.configService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, configs)
}

func (h *StorageHandler) GetAllEnabledStorageConfigs(c *gin.Context) {
	configs, err := h.configService.GetAllEnabled(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, configs)
}

func (h *StorageHandler) GetStorageConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	config, err := h.configService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, config)
}

func (h *StorageHandler) CreateStorageConfig(c *gin.Context) {
	var req storageDto.CreateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	userID, _ := c.Get("userID")
	operatorID := userID.(uint)

	id, err := h.configService.Create(c.Request.Context(), &req, operatorID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, gin.H{"id": id})
}

func (h *StorageHandler) UpdateStorageConfig(c *gin.Context) {
	var req storageDto.UpdateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	userID, _ := c.Get("userID")
	operatorID := userID.(uint)

	if err := h.configService.Update(c.Request.Context(), &req, operatorID); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

func (h *StorageHandler) DeleteStorageConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.configService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

func (h *StorageHandler) SetDefaultStorageConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.configService.SetDefault(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

func (h *StorageHandler) TestStorageUpload(c *gin.Context) {
	var req storageDto.TestUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	url, err := h.configService.TestUpload(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, gin.H{"url": url})
}

func (h *StorageHandler) GetUploadRecordList(c *gin.Context) {
	var req storageDto.RecordQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	records, total, err := h.recordService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, records)
}

func (h *StorageHandler) GetUploadRecord(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	record, err := h.recordService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, record)
}

func (h *StorageHandler) DeleteUploadRecord(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.recordService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

func (h *StorageHandler) DeleteUploadRecords(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.recordService.DeleteMultiple(c.Request.Context(), req.IDs); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

func (h *StorageHandler) GetUploadCredentials(c *gin.Context) {
	var req storageDto.GetCredentialsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	result, err := h.recordService.GetUploadCredentials(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

func (h *StorageHandler) CreateUploadRecord(c *gin.Context) {
	var req storageDto.CreateRecordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized)
		return
	}
	source := storageEntity.UploadSourceAdmin

	result, err := h.recordService.CreateUploadRecord(
		c.Request.Context(),
		&req,
		source,
		userID.(uint),
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}
