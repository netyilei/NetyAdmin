// adminapi.go：storage 管理端点（——配置 CRUD/设默认/记录列表/手动清理）。
package adapters

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/storage/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/storage/application"
	"github.com/netyilei/netyadmin/internal/modules/storage/domain"
)

// AdminDeps 是存储管理端点依赖。
type AdminDeps struct {
	Service    *application.Service
	Authorizer authz.Authorizer
	OnDeny     func(*gin.Context, authn.Principal, string, string)
}

// RegisterAdminRoutes 注册存储管理端点（module.go 调用）。
func RegisterAdminRoutes(authed gin.IRouter, deps AdminDeps) {
	authzMW := transport.AuthzMiddleware(deps.Authorizer, transport.PrincipalFrom, deps.OnDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}
	cfg := authed.Group("/storage/configs")
	cfg.GET("", secured(keys.PermConfigsList, listConfigsHandler(deps))...)
	cfg.POST("", secured(keys.PermConfigsWrite, createConfigHandler(deps))...)
	cfg.PUT("/:id", secured(keys.PermConfigsWrite, updateConfigHandler(deps))...)
	cfg.DELETE("/:id", secured(keys.PermConfigsWrite, deleteConfigHandler(deps))...)
	cfg.PUT("/:id/default", secured(keys.PermConfigsWrite, setDefaultHandler(deps))...)
	authed.GET("/storage/records", secured(keys.PermRecordsList, listRecordsHandler(deps))...)
	authed.POST("/storage/cleanup", secured(keys.PermCleanup, cleanupHandler(deps))...)
}

// @Summary 存储配置列表
// @Tags storage
// @ID admin.storage.configs.list
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} errorx.Envelope{data=transport.Page}
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/storage/configs [get]
func listConfigsHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, size := transport.BindPage(c)
		list, total, err := deps.Service.ListConfigs(c.Request.Context(), transport.Offset(page, size), size)
		if err != nil {
			transport.FailOf(c, mapStorageError(err))
			return
		}
		// Secret 出域面关闭：列表响应与 create/update 同掩码（create/update 单条
		// 已掩码——此处防 LIST 直通 repo 原始行泄露明文 secret_key）
		for i := range list {
			list[i] = maskSecret(list[i])
		}
		transport.OK(c, transport.NewPage(list, int64(total), page, size))
	}
}

// configRequest 是配置写请求（secret_key 写入后不回显——响应 Secret 面掩码）。
type configRequest struct {
	Name         string `json:"name" validate:"required"`
	Provider     string `json:"provider" validate:"required"`
	Endpoint     string `json:"endpoint" validate:"required"`
	Region       string `json:"region"`
	BucketLookup string `json:"bucket_lookup"` // 寻址风格：空/path（默认）/virtual-host（COS 等强制虚拟主机域名的 S3 兼容服务）
	Bucket       string `json:"bucket" validate:"required"`
	AccessKey    string `json:"access_key" validate:"required"`
	SecretKey    string `json:"secret_key" validate:"required"`
	Domain       string `json:"domain"`
	PathPrefix   string `json:"path_prefix"`
	IsDefault    bool   `json:"is_default"`
	Status       int16  `json:"status"`
	MaxFileSize  int64  `json:"max_file_size"`
	AllowedTypes string `json:"allowed_types"`
}

// @Summary 创建存储配置
// @Tags storage
// @ID admin.storage.configs.create
// @Accept json
// @Param body body configRequest true "配置"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/storage/configs [post]
func createConfigHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req configRequest
		if !transport.Bind(c, &req) {
			return
		}
		cfg := toDomainConfig(req)
		if cfg.Status == 0 {
			cfg.Status = 1 // 缺省=enabled（与 message createTemplate 同守卫——死配置防呆）
		}
		if err := deps.Service.CreateConfig(c.Request.Context(), cfg); err != nil {
			transport.FailOf(c, mapStorageError(err))
			return
		}
		transport.OK(c, maskSecret(cfg))
	}
}

// @Summary 更新存储配置
// @Tags storage
// @ID admin.storage.configs.update
// @Accept json
// @Param id path string true "配置 ID"
// @Param body body configRequest true "配置"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/storage/configs/{id} [put]
func updateConfigHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req configRequest
		if !transport.Bind(c, &req) {
			return
		}
		cfg := toDomainConfig(req)
		cfg.ID = c.Param("id")
		if err := deps.Service.UpdateConfig(c.Request.Context(), cfg); err != nil {
			transport.FailOf(c, mapStorageError(err))
			return
		}
		transport.OK(c, maskSecret(cfg))
	}
}

// @Summary 删除存储配置
// @Tags storage
// @ID admin.storage.configs.delete
// @Param id path string true "配置 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/storage/configs/{id} [delete]
func deleteConfigHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.DeleteConfig(c.Request.Context(), c.Param("id")); err != nil {
			transport.FailOf(c, mapStorageError(err))
			return
		}
		transport.OK(c, gin.H{})
	}
}

// @Summary 设默认存储配置
// @Tags storage
// @ID admin.storage.configs.default
// @Param id path string true "配置 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/storage/configs/{id}/default [put]
func setDefaultHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.SetDefault(c.Request.Context(), c.Param("id")); err != nil {
			transport.FailOf(c, mapStorageError(err))
			return
		}
		transport.OK(c, gin.H{})
	}
}

// @Summary 上传记录列表
// @Tags storage
// @ID admin.storage.records.list
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} errorx.Envelope{data=transport.Page}
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/storage/records [get]
func listRecordsHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, size := transport.BindPage(c)
		list, total, err := deps.Service.ListRecords(c.Request.Context(), transport.Offset(page, size), size)
		if err != nil {
			transport.FailOf(c, mapStorageError(err)) // 裸哨兵禁直传（C7——与 cleanup 同惯例）
			return
		}
		transport.OK(c, transport.NewPage(list, int64(total), page, size))
	}
}

// @Summary 手动清理过期上传记录
// @Tags storage
// @ID admin.storage.cleanup
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/storage/cleanup [post]
func cleanupHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		n, err := deps.Service.CleanupExpired(c.Request.Context())
		if err != nil {
			transport.FailOf(c, mapStorageError(err)) // 裸哨兵禁直传 FailOf（同 clientapi 惯例）
			return
		}
		transport.OK(c, gin.H{"expired": n})
	}
}

// mapStorageError 哨兵 → 业务码（AGENTS §6 单一源——禁裸哨兵归一 10009）。
func mapStorageError(err error) error {
	switch {
	case errors.Is(err, application.ErrConfigNotFound), errors.Is(err, application.ErrRecordNotFound):
		return errorx.New(errorx.CodeNotFound)
	case errors.Is(err, application.ErrConfigConflict):
		return errorx.New(errorx.CodeConflict)
	case errors.Is(err, application.ErrSecretMismatch):
		return errorx.New(errorx.CodeForbidden)
	case errors.Is(err, application.ErrAlreadyCompleted):
		return errorx.New(errorx.CodeConflict)
	case errors.Is(err, application.ErrNoDefaultConfig), errors.Is(err, application.ErrExpiredWindow),
		errors.Is(err, application.ErrConfigDisabled), errors.Is(err, application.ErrHMACNotConfigured):
		return errorx.New(errorx.CodeUnavailable)
	case errors.Is(err, domain.ErrProviderInvalid), errors.Is(err, domain.ErrBucketLookupInvalid),
		errors.Is(err, domain.ErrPathPrefixInvalid):
		return errorx.New(errorx.CodeValidationFailed)
	}
	return err
}

func toDomainConfig(req configRequest) *domain.StorageConfig {
	return &domain.StorageConfig{
		Name: req.Name, Provider: domain.StorageProvider(req.Provider),
		Endpoint: req.Endpoint, Region: req.Region,
		BucketLookup: domain.StorageBucketLookup(req.BucketLookup), Bucket: req.Bucket,
		AccessKey: req.AccessKey, SecretKey: req.SecretKey, Domain: req.Domain,
		PathPrefix: req.PathPrefix, IsDefault: req.IsDefault,
		Status: req.Status, MaxFileSize: req.MaxFileSize, AllowedTypes: req.AllowedTypes,
	}
}

// maskSecret 返回脱敏副本（响应面不回显 secret 原文——保留长度提示；
// ≤4 字符全掩码防短值反推；不改入参——副本语义杜绝跨响应污染）。
func maskSecret(cfg *domain.StorageConfig) *domain.StorageConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	if len(out.SecretKey) > 4 {
		out.SecretKey = "****" + out.SecretKey[len(out.SecretKey)-4:]
	} else if out.SecretKey != "" {
		out.SecretKey = "****"
	}
	return &out
}
