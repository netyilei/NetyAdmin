// adminapi.go：配置管理 HTTP 端点（V1 3 端点对齐——public/list/update；
// V1 test-email 不做：外部 SMTP 消费者无，登记随具名消费者）。
// 模块自注册模式：cmd 组装经 Module.RegisterAdminRoutes 挂载——
// public 面免鉴权（前端登录页读公开键），管理面走 authed 链（权限码声明）。
package adapters

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/application"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/domain"
)

// Deps 是 handlers 依赖（module.go 装配注入）。
type Deps struct {
	Service    *application.Service
	Authorizer authz.Authorizer // RBAC 判定面（cmd 注入；nil=fail-closed 全拒）
	OnDeny     func(*gin.Context, authn.Principal, string, string)
}

// RegisterAdminRoutes 注册配置端点（module.go 调用）。
func RegisterAdminRoutes(public, authed gin.IRouter, deps Deps) {
	authzMW := transport.AuthzMiddleware(deps.Authorizer, transport.PrincipalFrom, deps.OnDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}

	public.GET("/configs/public", publicConfigsHandler(deps))

	configs := authed.Group("/configs")
	configs.GET("", secured(keys.PermConfigsList, listConfigsHandler(deps))...)
	configs.PUT("", secured(keys.PermConfigsUpdate, updateConfigsHandler(deps))...)
	configs.DELETE("/:group/:key", secured(keys.PermConfigsUpdate, deleteConfigHandler(deps))...) // 单键删除（BatchUpsert 纯 upsert 不删缺行——删除独立面）
	configs.POST("/rollback", secured(keys.PermConfigsRollback, rollbackHandler(deps))...)        // 装-A2 清偿
	configs.GET("/history", secured(keys.PermConfigsList, historyHandler(deps))...)
}

// @Summary 公开配置
// @Description is_public 键集（免鉴权——前端登录页/品牌参数）
// @Tags configs
// @ID admin.configs.public
// @Success 200 {object} errorx.Envelope
// @Router /admin/v1/configs/public [get]
func publicConfigsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := deps.Service.Public(c.Request.Context())
		if err != nil {
			writeConfigError(c, err)
			return
		}
		transport.OK(c, list)
	}
}

// @Summary 配置全量列表
// @Tags configs
// @ID admin.configs.list
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/configs [get]
func listConfigsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := deps.Service.All(c.Request.Context())
		if err != nil {
			writeConfigError(c, err)
			return
		}
		transport.OK(c, list)
	}
}

type updateItem struct {
	GroupName   string `json:"group" validate:"required,min=1,max=100"`
	ConfigKey   string `json:"key" validate:"required,min=1,max=100"`
	ConfigValue string `json:"value" validate:"max=4096"`
	ValueType   int    `json:"valueType" validate:"required,gte=1,lte=4"`
	Description string `json:"description" validate:"max=255"`
	IsPublic    bool   `json:"isPublic"`
}

type updateConfigsRequest struct {
	Items  []updateItem `json:"items" validate:"required,gt=0,dive"`
	Reason string       `json:"reason" validate:"max=255"`
}

// @Summary 批量更新配置
// @Description 全量提交变更键集（V1 同语义——PUT 覆盖值/描述/公开位；变更落
// 历史 + 广播热更）；新建键用 items 内新 group+key（upsert 语义统一入口）
// @Tags configs
// @ID admin.configs.update
// @Accept json
// @Param body body updateConfigsRequest true "变更集与说明"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/configs [put]
func updateConfigsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		var req updateConfigsRequest
		if !transport.Bind(c, &req) {
			return
		}
		// 整批一次变更（评审修：单版本/单快照/单广播；upsert 语义统一）。
		items := make([]domain.Config, 0, len(req.Items))
		for _, it := range req.Items {
			items = append(items, domain.Config{
				GroupName: it.GroupName, ConfigKey: it.ConfigKey,
				ConfigValue: it.ConfigValue, ValueType: domain.ValueType(it.ValueType),
				Description: it.Description, IsPublic: it.IsPublic,
			})
		}
		if err := deps.Service.BatchUpsert(c.Request.Context(), items, p.ID, req.Reason); err != nil {
			writeConfigError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// writeConfigError 领域错误映射（errorx 信封）。
func writeConfigError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		transport.Fail(c, errorx.New(errorx.CodeNotFound))
	case errors.Is(err, domain.ErrKeyExists):
		transport.Fail(c, errorx.New(errorx.CodeConflict, "reason", "key exists"))
	case errors.Is(err, domain.ErrSystemProtected):
		transport.Fail(c, errorx.New(errorx.CodeForbidden))
	case errors.Is(err, domain.ErrInvalidValue):
		transport.Fail(c, errorx.New(errorx.CodeValidationFailed))
	case errors.Is(err, domain.ErrInvalidKey):
		transport.Fail(c, errorx.New(errorx.CodeValidationFailed, "reason", "invalid key charset"))
	case errors.Is(err, domain.ErrReservedGroup):
		transport.Fail(c, errorx.New(errorx.CodeForbidden, "reason", "reserved group"))
	case errors.Is(err, domain.ErrTooManyKeys):
		// 键集超量级上限→ 400 校验失败（413 仅留给传输层
		// body 超限中间件——语义分层：这里被拒的是键集规模非请求体积）。
		transport.Fail(c, errorx.New(errorx.CodeValidationFailed,
			"reason", fmt.Sprintf("key set exceeds cap %d", domain.MaxTotalKeys)))
	default:
		transport.FailOf(c, err)
	}
}

// @Summary 删除配置键
// @Description 单键删除（独立于 PUT 全量提交——BatchUpsert 纯 upsert 不删缺行；
// 系统保护键删除被拒）；变更落历史可回滚
// @Tags configs
// @ID admin.configs.delete
// @Param group path string true "分组"
// @Param key path string true "键"
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/configs/{group}/{key} [delete]
func deleteConfigHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		if err := deps.Service.Delete(c.Request.Context(), c.Param("group"), c.Param("key"), p.ID, "管理端删除配置键"); err != nil {
			writeConfigError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// @Summary 回滚到目标版本前状态
// @Description 恢复目标版本之前的键集（机制面 Rollback——管理面端点补建）
// @Tags configs
// @ID admin.configs.rollback
// @Accept json
// @Param body body rollbackRequest true "回滚请求"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/configs/rollback [post]
func rollbackHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req rollbackRequest
		if !transport.Bind(c, &req) {
			return
		}
		p := transport.PrincipalFrom(c)
		if err := deps.Service.Rollback(c.Request.Context(), req.Version, p.ID); err != nil {
			transport.FailOf(c, err)
			return
		}
		transport.OK(c, gin.H{"version": req.Version})
	}
}

// rollbackRequest 是回滚请求（version=目标历史版本号——恢复该版本前状态）。
type rollbackRequest struct {
	Version int64 `json:"version" validate:"required,gt=0"`
}

// @Summary 变更历史（最近 50 版本）
// @Tags configs
// @ID admin.configs.history
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/configs/history [get]
func historyHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := deps.Service.ListHistory(c.Request.Context())
		if err != nil {
			writeConfigError(c, err)
			return
		}
		transport.OK(c, list)
	}
}
