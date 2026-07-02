package storage

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	storageEntity "NetyAdmin/internal/domain/entity/storage"
	storageDto "NetyAdmin/internal/interface/admin/dto/storage"
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

// @Summary      获取存储配置列表
// @Description  分页获取存储配置列表
// @Tags         存储管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Success      200 {object} response.Response "存储配置列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage-configs [get]
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

// @Summary      获取所有启用的存储配置
// @Description  获取所有状态为启用的存储配置列表
// @Tags         存储管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "存储配置列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage-configs/all-enabled [get]
func (h *StorageHandler) GetAllEnabledStorageConfigs(c *gin.Context) {
	configs, err := h.configService.GetAllEnabled(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, configs)
}

// @Summary      获取存储配置详情
// @Description  根据ID获取单个存储配置详情
// @Tags         存储管理
// @Accept       json
// @Produce      json
// @Param        id path int true "存储配置ID"
// @Success      200 {object} response.Response "存储配置详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage-configs/{id} [get]
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

// @Summary      创建存储配置
// @Description  新建一个存储配置
// @Tags         存储管理
// @Accept       json
// @Produce      json
// @Param        req body storage.CreateConfigReq true "创建存储配置参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage-configs [post]
func (h *StorageHandler) CreateStorageConfig(c *gin.Context) {
	var req storageDto.CreateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	id, err := h.configService.Create(c.Request.Context(), &req, operatorID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, gin.H{"id": id})
}

// @Summary      更新存储配置
// @Description  更新存储配置信息
// @Tags         存储管理
// @Accept       json
// @Produce      json
// @Param        req body storage.UpdateConfigReq true "更新存储配置参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage-configs [put]
func (h *StorageHandler) UpdateStorageConfig(c *gin.Context) {
	var req storageDto.UpdateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	if err := h.configService.Update(c.Request.Context(), &req, operatorID); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      删除存储配置
// @Description  根据ID删除存储配置
// @Tags         存储管理
// @Accept       json
// @Produce      json
// @Param        id path int true "存储配置ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage-configs/{id} [delete]
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

// @Summary      设置默认存储配置
// @Description  根据ID将存储配置设为默认
// @Tags         存储管理
// @Accept       json
// @Produce      json
// @Param        id path int true "存储配置ID"
// @Success      200 {object} response.Response "设置成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage-configs/{id}/default [put]
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

// @Summary      测试存储上传
// @Description  使用指定存储配置测试文件上传
// @Tags         存储管理
// @Accept       json
// @Produce      json
// @Param        req body storage.TestUploadReq true "测试上传参数"
// @Success      200 {object} response.Response "测试结果"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage-configs/test-upload [post]
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

// @Summary      获取上传记录列表
// @Description  分页获取上传记录列表，支持按文件名、来源、业务类型、MIME类型、存储配置、应用、时间范围筛选
// @Tags         上传记录管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        fileName query string false "文件名"
// @Param        source query string false "来源"
// @Param        sourceId query string false "来源ID"
// @Param        businessType query string false "业务类型"
// @Param        businessId query string false "业务ID"
// @Param        mimeType query string false "MIME类型"
// @Param        storageConfigId query int false "存储配置ID"
// @Param        appId query string false "应用ID"
// @Param        startTime query string false "开始时间"
// @Param        endTime query string false "结束时间"
// @Success      200 {object} response.Response "上传记录列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/upload-records [get]
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

// @Summary      获取上传记录详情
// @Description  根据ID获取单个上传记录详情
// @Tags         上传记录管理
// @Accept       json
// @Produce      json
// @Param        id path int true "上传记录ID"
// @Success      200 {object} response.Response "上传记录详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/upload-records/{id} [get]
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

// @Summary      删除上传记录
// @Description  根据ID删除上传记录
// @Tags         上传记录管理
// @Accept       json
// @Produce      json
// @Param        id path int true "上传记录ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/upload-records/{id} [delete]
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

// @Summary      批量删除上传记录
// @Description  根据ID数组批量删除上传记录
// @Tags         上传记录管理
// @Accept       json
// @Produce      json
// @Param        req body object true "批量删除参数，字段: ids(整型数组)"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/upload-records/batch-delete [post]
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

// @Summary      获取上传凭证
// @Description  获取文件上传凭证(含recordId与secret，用于上传成功通知验签)
// @Tags         文件上传
// @Accept       json
// @Produce      json
// @Param        req body storage.GetCredentialsReq true "获取上传凭证参数"
// @Success      200 {object} response.Response "上传凭证"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage/upload-credentials [post]
func (h *StorageHandler) GetUploadCredentials(c *gin.Context) {
	var req storageDto.GetCredentialsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	sourceID := fmt.Sprintf("%d", c.GetUint("adminID"))

	result, err := h.recordService.GetUploadCredentials(c.Request.Context(), &req, "", storageEntity.UploadSourceAdmin, sourceID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

// CreateUploadRecord 上传成功通知：根据 recordId + secret 校验后将 pending 记录置为 uploaded。
// @Summary      上传成功通知
// @Description  上传成功后通知服务端，根据 recordId 与 secret 校验后将 pending 记录置为 uploaded
// @Tags         文件上传
// @Accept       json
// @Produce      json
// @Param        req body storage.CompleteUploadReq true "上传成功通知参数"
// @Success      200 {object} response.Response "上传记录"
// @Security    ApiKeyAuth
// @Router       /admin/v1/storage/upload-record [post]
func (h *StorageHandler) CreateUploadRecord(c *gin.Context) {
	var req storageDto.CompleteUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	result, err := h.recordService.CompleteUpload(
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
