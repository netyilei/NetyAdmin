// adminapi.go：gw 治理管理端点（V1 open_platform admin 26 端点 RESTful
// 收敛为 16——apps CRUD 5+重置密钥+IP 策略+授权 2+scope 组列表+APIs CRUD 4+
// scope-apis 读/写；open logs 统计随）。模块自注册。
package adapters

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/gw/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/gw/application"
	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// Deps 是 handlers 依赖（module.go 装配注入）。
type Deps struct {
	Service    *application.Service
	Authorizer authz.Authorizer
	OnDeny     func(*gin.Context, authn.Principal, string, string) // Stats 是网关统计面（——nil=统计端点缺席）。
	Stats      *application.StatsService
	IPAC       *application.IPACManager // IP 策略组/规则管理
	Engine     *application.IPEngine    // IP 判定引擎（reload 面）
	Pool       *database.Pool
}

// securedChain 是子文件共用签名（对照 adminapi_roles.go 等子文件）。
type securedChain = func(perm string, h gin.HandlerFunc) []gin.HandlerFunc

// updateIPGroupHandler 更新组。
//
// @Summary 更新 IP 策略组
// @Tags open-ip
// @ID admin.gw.ip-groups.update
// @Accept json
// @Param id path string true "组 ID"
// @Param body body ipGroupRequest true "组"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/open/ip-groups/{id} [put]
func updateIPGroupHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ipGroupRequest
		if !transport.Bind(c, &req) {
			return
		}
		g := domain.IPGroup{ID: c.Param("id"), Name: req.Name, Remark: req.Remark}
		if err := deps.IPAC.UpdateGroup(c.Request.Context(), &g); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, g)
	}
}

// deleteIPGroupHandler 删除组（规则随 FK CASCADE；应用组集引用中=后端
// ErrIPGroupInUse 硬守卫 409——前端引导先解绑）。
//
// @Summary 删除 IP 策略组
// @Tags open-ip
// @ID admin.gw.ip-groups.delete
// @Param id path string true "组 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/open/ip-groups/{id} [delete]
func deleteIPGroupHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.IPAC.DeleteGroup(c.Request.Context(), c.Param("id")); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// RegisterAdminRoutes 注册治理端点（module.go 调用）。
func RegisterAdminRoutes(authed gin.IRouter, deps Deps) {
	authzMW := transport.AuthzMiddleware(deps.Authorizer, transport.PrincipalFrom, deps.OnDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}
	registerIPAdminRoutes(authed, deps, secured)

	apps := authed.Group("/open/apps")
	apps.GET("", secured(keys.PermAppsList, listAppsHandler(deps))...)
	apps.POST("", secured(keys.PermAppsCreate, createAppHandler(deps))...)
	apps.PUT("/:id", secured(keys.PermAppsUpdate, updateAppHandler(deps))...)
	apps.DELETE("/:id", secured(keys.PermAppsDelete, deleteAppHandler(deps))...)
	apps.PUT("/:id/reset-secret", secured(keys.PermAppsResetSecret, resetSecretHandler(deps))...)
	apps.PUT("/:id/status", secured(keys.PermAppsUpdate, setAppStatusHandler(deps))...)
	apps.GET("/available-scopes", secured(keys.PermAppsList, availableScopesHandler(deps))...)

	// 接口权限页（scope 组字典管理——应用侧 scopes 并入 UpdateApp 后无独立授权端点）
	sgs := authed.Group("/open/scope-groups")
	sgs.GET("", secured(keys.PermScopeGroupsList, listScopeGroupsHandler(deps))...)
	sgs.POST("", secured(keys.PermScopeGroupsWrite, createScopeGroupHandler(deps))...)
	sgs.PUT("/:id", secured(keys.PermScopeGroupsWrite, updateScopeGroupHandler(deps))...)
	sgs.DELETE("/:id", secured(keys.PermScopeGroupsWrite, deleteScopeGroupHandler(deps))...)

	apis := authed.Group("/open/apis")
	apis.GET("", secured(keys.PermApisList, listApisHandler(deps))...)
	apis.POST("", secured(keys.PermApisCreate, createApiHandler(deps))...)
	apis.PUT("/:id", secured(keys.PermApisUpdate, updateApiHandler(deps))...)
	apis.DELETE("/:id", secured(keys.PermApisDelete, deleteApiHandler(deps))...)
	apis.GET("/scope-apis", secured(keys.PermApisList, scopeApisHandler(deps))...)
	apis.PUT("/scope-apis", secured(keys.PermApisBind, setScopeApisHandler(deps))...)

	// 统计面（流水/快照全文件化 2026-11-19——端点读内存实时窗口
	// +本机全部节点 gw_stats_hourly_<node>.jsonl 快照历史合并；零 DB）
	if deps.Stats != nil {
		st := authed.Group("/open/stats")
		st.GET("/overview", secured(keys.PermStatsView, statsOverviewHandler(deps))...)
		st.GET("/trend", secured(keys.PermStatsView, statsTrendHandler(deps))...)
		st.GET("/top", secured(keys.PermStatsView, statsTopHandler(deps))...)
		st.GET("/status", secured(keys.PermStatsView, statsStatusHandler(deps))...)
		st.GET("/latency", secured(keys.PermStatsView, statsLatencyHandler(deps))...)
	}
}

// —— Stats——

// statsHours 取查询窗口（小时；默认 24 上限 720——query 解析，
// 此前误读 gin context key（无 set 方）恒默认 24，时间窗切换无效）。
func statsHours(c *gin.Context) int {
	h, _ := strconv.Atoi(c.Query("hours"))
	if h <= 0 {
		h = 24
	}
	if h > 720 {
		h = 720
	}
	return h
}

// @Summary 网关调用总览
// @Tags open-stats
// @ID admin.gw.stats.overview
// @Produce json
// @Param hours query int false "回看窗口小时数（默认 24 上限 720）"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/stats/overview [get]
func statsOverviewHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		transport.OK(c, deps.Stats.Overview(statsHours(c)))
	}
}

// @Summary 网关调用趋势（小时粒度）
// @Tags open-stats
// @ID admin.gw.stats.trend
// @Produce json
// @Param hours query int false "回看窗口小时数（默认 24 上限 720）"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/stats/trend [get]
func statsTrendHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		transport.OK(c, deps.Stats.Trend(statsHours(c)))
	}
}

// @Summary 网关调用 Top 榜（apps|apis）
// @Tags open-stats
// @ID admin.gw.stats.top
// @Produce json
// @Param kind query string true "apps|apis"
// @Param limit query int false "条数（默认 10）"
// @Param hours query int false "回看窗口小时数（默认 24 上限 720）"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/stats/top [get]
func statsTopHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		kind := c.DefaultQuery("kind", "apps")
		if kind != "apps" && kind != "apis" {
			transport.Fail(c, errorx.New(errorx.CodeInvalidParams, "reason", "kind"))
			return
		}
		limit, _ := strconv.Atoi(c.Query("limit")) // 缺省/非法→默认 10
		if limit <= 0 {
			limit = 10
		}
		transport.OK(c, deps.Stats.Top(kind, limit, statsHours(c)))
	}
}

// @Summary 网关调用状态码分布
// @Tags open-stats
// @ID admin.gw.stats.status
// @Produce json
// @Param hours query int false "回看窗口小时数（默认 24 上限 720）"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/stats/status [get]
func statsStatusHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		transport.OK(c, deps.Stats.StatusDist(statsHours(c)))
	}
}

// @Summary 网关调用延迟分位（avg/p50/p95）
// @Tags open-stats
// @ID admin.gw.stats.latency
// @Produce json
// @Param hours query int false "回看窗口小时数（默认 24 上限 720）"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/stats/latency [get]
func statsLatencyHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		transport.OK(c, deps.Stats.Latency(statsHours(c)))
	}
}

// —— Apps ——

// @Summary 开放平台应用列表
// @Tags open-apps
// @ID admin.gw.apps.list
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} errorx.Envelope{data=transport.Page}
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/apps [get]
func listAppsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, size := transport.BindPage(c)
		list, total, err := deps.Service.ListApps(c.Request.Context(), transport.Offset(page, size), size)
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, transport.NewPage(list, total, page, size))
	}
}

type createAppRequest struct {
	Name       string             `json:"name" validate:"required,max=100"`
	AppKey     string             `json:"appKey" validate:"required,min=16,max=64"`
	Secret     string             `json:"secret" validate:"required,min=16,max=128"`
	Remark     string             `json:"remark" validate:"max=255"`
	StorageRef string             `json:"storageRef" validate:"max=64"`
	Quota      domain.QuotaConfig `json:"quota"`
	Scopes     []string           `json:"scopes" validate:"dive,max=50"`
	IPGroupIDs []string           `json:"ipGroupIds" validate:"dive,max=64"` // IP 策略组集
	RateLimit  *bool              `json:"rateLimitEnabled"`                  // nil=默认开
}

// @Summary 创建开放平台应用
// @Description AppSecret 明文一次性返回（响应通道唯一）
// @Tags open-apps
// @ID admin.gw.apps.create
// @Accept json
// @Param body body createAppRequest true "应用定义"
// @Success 200 {object} errorx.Envelope{data=domain.AppView}
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/open/apps [post]
func createAppHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createAppRequest
		if !transport.Bind(c, &req) {
			return
		}
		view, err := deps.Service.CreateApp(c.Request.Context(), req.Name, req.AppKey,
			req.Secret, req.Remark, req.StorageRef, req.Quota, req.Scopes, req.IPGroupIDs, req.RateLimit)
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, view)
	}
}

// updateAppRequest 是部分更新面（评审修：指针字段 nil=不改——
// 此前零值静默覆盖=漏传 quota 即关限流；status 走独立 /status 端点）。
type updateAppRequest struct {
	Name             *string             `json:"name" validate:"omitempty,max=100"`
	Remark           *string             `json:"remark" validate:"omitempty,max=255"`
	StorageRef       *string             `json:"storageRef" validate:"omitempty,max=64"`
	Quota            *domain.QuotaConfig `json:"quota"`
	RateLimitEnabled *bool               `json:"rateLimitEnabled"`
	IPGroupIDs       *[]string           `json:"ipGroupIds" validate:"omitempty,dive,max=64"` // 非 nil=组集全量替换
	Scopes           *[]string           `json:"scopes" validate:"omitempty,dive,max=50"`     // 非 nil=授权全量替换（表单单一保存入口）
}

// @Summary 更新开放平台应用
// @Tags open-apps
// @ID admin.gw.apps.update
// @Accept json
// @Param id path string true "应用 ID"
// @Param body body updateAppRequest true "更新字段"
// @Success 200 {object} errorx.Envelope{data=domain.AppView}
// @Failure 400 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/open/apps/{id} [put]
func updateAppHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updateAppRequest
		if !transport.Bind(c, &req) {
			return
		}
		view, err := deps.Service.UpdateApp(c.Request.Context(), c.Param("id"),
			req.Name, req.Remark, req.StorageRef, req.Quota, req.RateLimitEnabled,
			req.IPGroupIDs, req.Scopes)
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, view)
	}
}

// @Summary 删除开放平台应用
// @Tags open-apps
// @ID admin.gw.apps.delete
// @Param id path string true "应用 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/open/apps/{id} [delete]
func deleteAppHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.DeleteApp(c.Request.Context(), c.Param("id")); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

type secretResult struct {
	AppKey string `json:"appKey"`
	Secret string `json:"secret"` // 一次性交付（响应通道唯一）
}

// @Summary 重置 AppSecret
// @Description 新密钥明文一次性返回；旧密钥立即失效
// @Tags open-apps
// @ID admin.gw.apps.reset_secret
// @Param id path string true "应用 ID"
// @Success 200 {object} errorx.Envelope{data=secretResult}
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/open/apps/{id}/reset-secret [put]
func resetSecretHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		view, err := deps.Service.GetApp(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeGwError(c, err)
			return
		}
		secret, err := deps.Service.ResetAppSecret(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, secretResult{AppKey: view.AppKey, Secret: secret})
	}
}

type appStatusRequest struct {
	Status int `json:"status" validate:"required,eq=1|eq=2"`
}

// @Summary 应用启停
// @Tags open-apps
// @ID admin.gw.apps.status
// @Accept json
// @Param id path string true "应用 ID"
// @Param body body appStatusRequest true "状态"
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/apps/{id}/status [put]
func setAppStatusHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req appStatusRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := deps.Service.SetAppStatus(c.Request.Context(),
			c.Param("id"), domain.Status(req.Status)); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// @Summary 可用 scope 组清单
// @Tags open-apps
// @ID admin.gw.apps.available_scopes
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/apps/available-scopes [get]
func availableScopesHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := deps.Service.ListScopeGroups(c.Request.Context())
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, groups)
	}
}

// —— Scope groups 管理面（接口权限页——组字典 CRUD） ——

// @Summary scope 组列表
// @Tags open-scope-groups
// @ID admin.gw.scope-groups.list
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/scope-groups [get]
func listScopeGroupsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := deps.Service.ListScopeGroups(c.Request.Context())
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, groups)
	}
}

type scopeGroupRequest struct {
	Code        string `json:"code" validate:"required,min=2,max=50"`
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"max=255"`
	Status      int    `json:"status" validate:"omitempty,eq=1|eq=2"`
}

// @Summary 创建 scope 组
// @Tags open-scope-groups
// @ID admin.gw.scope-groups.create
// @Accept json
// @Param body body scopeGroupRequest true "scope 组"
// @Success 200 {object} errorx.Envelope{data=domain.ScopeGroup}
// @Failure 400 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/open/scope-groups [post]
func createScopeGroupHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req scopeGroupRequest
		if !transport.Bind(c, &req) {
			return
		}
		g, err := deps.Service.CreateScopeGroup(c.Request.Context(), domain.ScopeGroup{
			Code: req.Code, Name: req.Name, Description: req.Description, Status: domain.Status(req.Status),
		})
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, g)
	}
}

// @Summary 更新 scope 组（code 不可变——授权引用按 code）
// @Tags open-scope-groups
// @ID admin.gw.scope-groups.update
// @Accept json
// @Param id path string true "scope 组 ID"
// @Param body body scopeGroupRequest true "scope 组"
// @Success 200 {object} errorx.Envelope{data=domain.ScopeGroup}
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/open/scope-groups/{id} [put]
func updateScopeGroupHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req scopeGroupRequest
		if !transport.Bind(c, &req) {
			return
		}
		g, err := deps.Service.UpdateScopeGroup(c.Request.Context(), domain.ScopeGroup{
			ID: c.Param("id"), Name: req.Name, Description: req.Description, Status: domain.Status(req.Status),
		})
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, g)
	}
}

// @Summary 删除 scope 组（应用授权或 API 白名单引用中=409）
// @Tags open-scope-groups
// @ID admin.gw.scope-groups.delete
// @Param id path string true "scope 组 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/open/scope-groups/{id} [delete]
func deleteScopeGroupHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.DeleteScopeGroup(c.Request.Context(), c.Param("id")); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// —— APIs ——

// @Summary 网关 API 登记全量列表
// @Tags open-apis
// @ID admin.gw.apis.list
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/apis [get]
func listApisHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		apis, err := deps.Service.ListApis(c.Request.Context())
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, apis)
	}
}

type apiRequest struct {
	Method      string `json:"method" validate:"required,oneof=GET POST PUT PATCH DELETE"`
	Path        string `json:"path" validate:"required,max=255"`
	Name        string `json:"name" validate:"required,max=100"`
	Grp         string `json:"grp" validate:"max=50"`
	Description string `json:"description" validate:"max=255"`
	Status      int    `json:"status" validate:"omitempty,eq=1|eq=2"`
}

// @Summary 登记网关 API
// @Tags open-apis
// @ID admin.gw.apis.create
// @Accept json
// @Param body body apiRequest true "API 定义"
// @Success 200 {object} errorx.Envelope{data=domain.Api}
// @Failure 400 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/apis [post]
func createApiHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apiRequest
		if !transport.Bind(c, &req) {
			return
		}
		a, err := deps.Service.CreateApi(c.Request.Context(), domain.Api{
			Method: req.Method, Path: req.Path, Name: req.Name,
			Grp: req.Grp, Description: req.Description, Status: domain.Status(req.Status),
		})
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, a)
	}
}

// @Summary 更新网关 API
// @Tags open-apis
// @ID admin.gw.apis.update
// @Accept json
// @Param id path string true "API ID"
// @Param body body apiRequest true "API 定义"
// @Success 200 {object} errorx.Envelope{data=domain.Api}
// @Failure 400 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/apis/{id} [put]
func updateApiHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apiRequest
		if !transport.Bind(c, &req) {
			return
		}
		a, err := deps.Service.UpdateApi(c.Request.Context(), domain.Api{
			ID: c.Param("id"), Method: req.Method, Path: req.Path, Name: req.Name,
			Grp: req.Grp, Description: req.Description, Status: domain.Status(req.Status),
		})
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, a)
	}
}

// @Summary 删除网关 API
// @Tags open-apis
// @ID admin.gw.apis.delete
// @Param id path string true "API ID"
// @Success 200 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/open/apis/{id} [delete]
func deleteApiHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.DeleteApi(c.Request.Context(), c.Param("id")); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// @Summary scope 白名单读
// @Tags open-apis
// @ID admin.gw.apis.scope_apis_get
// @Param scope query string true "scope code"
// @Success 200 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/open/apis/scope-apis [get]
func scopeApisHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		apis, err := deps.Service.ScopeApis(c.Request.Context(), c.Query("scope"))
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, apis)
	}
}

type setScopeApisRequest struct {
	Scope string   `json:"scope" validate:"required,max=50"`
	APIs  []string `json:"apis" validate:"dive,required"`
}

// @Summary scope 白名单全量替换
// @Tags open-apis
// @ID admin.gw.apis.scope_apis_set
// @Accept json
// @Param body body setScopeApisRequest true "scope 与 API ID 全集"
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/apis/scope-apis [put]
func setScopeApisHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req setScopeApisRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := deps.Service.SetScopeApis(c.Request.Context(), req.Scope, req.APIs); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// writeGwError 领域错误映射（errorx 信封）。
func writeGwError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrIPGroupNotFound),
		errors.Is(err, domain.ErrIPRuleNotFound):
		transport.Fail(c, errorx.New(errorx.CodeNotFound))
	case errors.Is(err, application.ErrAppKeyTaken):
		transport.Fail(c, errorx.New(errorx.CodeConflict, "reason", "app key taken"))
	case errors.Is(err, application.ErrScopeNotGroup):
		transport.Fail(c, errorx.New(errorx.CodeValidationFailed, "reason", "unknown scope"))
	case errors.Is(err, domain.ErrDisabled):
		transport.Fail(c, errorx.New(errorx.CodeForbidden))
	case errors.Is(err, domain.ErrIPGroupNameTaken):
		transport.Fail(c, errorx.New(errorx.CodeConflict, "reason", "ip group name taken"))
	case errors.Is(err, domain.ErrIPGroupInUse):
		transport.Fail(c, errorx.New(errorx.CodeConflict, "reason", "ip group in use by apps"))
	case errors.Is(err, domain.ErrIPActionInvalid):
		transport.Fail(c, errorx.New(errorx.CodeValidationFailed, "reason", "ip action invalid"))
	case errors.Is(err, domain.ErrIPCIDRInvalid):
		transport.Fail(c, errorx.New(errorx.CodeValidationFailed, "reason", "ip cidr invalid"))
	case errors.Is(err, application.ErrScopeNotFound):
		transport.Fail(c, errorx.New(errorx.CodeNotFound, "reason", "scope group not found"))
	case errors.Is(err, application.ErrScopeCodeTaken):
		transport.Fail(c, errorx.New(errorx.CodeConflict, "reason", "scope code taken"))
	case errors.Is(err, application.ErrScopeGroupInUse):
		transport.Fail(c, errorx.New(errorx.CodeConflict, "reason", "scope group in use by apps"))
	case errors.Is(err, application.ErrScopeInvalid):
		transport.Fail(c, errorx.New(errorx.CodeValidationFailed, "reason", "scope group invalid"))
	case errors.Is(err, application.ErrScopeDisabled):
		transport.Fail(c, errorx.New(errorx.CodeValidationFailed, "reason", "scope group disabled"))
	case errors.Is(err, application.ErrBuiltInApp):
		transport.Fail(c, errorx.New(errorx.CodeForbidden, "reason", "builtin app immutable"))
	case errors.Is(err, application.ErrIPGroupsUnwired):
		transport.Fail(c, errorx.New(errorx.CodeUnavailable, "reason", "ip groups unwired"))
	case errors.Is(err, application.ErrIPGroupIDEmpty):
		transport.Fail(c, errorx.New(errorx.CodeValidationFailed, "reason", "ip group id empty"))
	default:
		transport.FailOf(c, err)
	}
}

// —— IP 访问控制管理端点（配置组——组/规则 CRUD；变更触发引擎 Reload
// + Bus 广播全节点；权限码 admin.gw.ip-rules.*——12570 种子同源） ——

// @Summary IP 策略组列表
// @Tags open-ip
// @ID admin.gw.ip-groups.list
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/ip-groups [get]
func listIPGroupsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := deps.IPAC.ListGroups(c.Request.Context())
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, groups)
	}
}

type ipGroupRequest struct {
	Name   string `json:"name" validate:"required,max=100"`
	Remark string `json:"remark" validate:"max=255"`
}

// @Summary 创建 IP 策略组
// @Tags open-ip
// @ID admin.gw.ip-groups.create
// @Accept json
// @Param body body ipGroupRequest true "组"
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/ip-groups [post]
func createIPGroupHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ipGroupRequest
		if !transport.Bind(c, &req) {
			return
		}
		g := domain.IPGroup{ID: primitives.NewULID(primitives.SystemClock{}), Name: req.Name, Remark: req.Remark}
		if err := deps.IPAC.InsertGroup(c.Request.Context(), &g); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, g)
	}
}

type ipRuleRequest struct {
	GroupID  string `json:"groupId" validate:"required"`
	CIDRs    string `json:"cidrs" validate:"required"`
	Action   int16  `json:"action" validate:"required,eq=1|eq=2"`
	Priority int    `json:"priority"`
	Enabled  *bool  `json:"enabled"`
	Remark   string `json:"remark" validate:"max=255"`
}

// @Summary IP 规则列表（按组）
// @Tags open-ip
// @ID admin.gw.ip-rules.list
// @Param groupId query string true "组 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/ip-rules [get]
func listIPRulesHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := c.Query("groupId")
		if groupID == "" {
			transport.Fail(c, errorx.New(errorx.CodeValidationFailed, "reason", "groupId"))
			return
		}
		rules, err := deps.IPAC.ListRulesByGroup(c.Request.Context(), groupID)
		if err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, rules)
	}
}

// @Summary 创建 IP 规则
// @Tags open-ip
// @ID admin.gw.ip-rules.create
// @Accept json
// @Param body body ipRuleRequest true "规则"
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/ip-rules [post]
func createIPRuleHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ipRuleRequest
		if !transport.Bind(c, &req) {
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		r := domain.IPRule{ID: primitives.NewULID(primitives.SystemClock{}), GroupID: req.GroupID,
			CIDRs: req.CIDRs, Action: domain.IPAction(req.Action), Priority: req.Priority,
			Enabled: enabled, Remark: req.Remark}
		if err := deps.IPAC.CreateRule(c.Request.Context(), &r, req.CIDRs); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, r)
	}
}

// @Summary 删除 IP 规则
// @Tags open-ip
// @ID admin.gw.ip-rules.delete
// @Param id path string true "规则 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/ip-rules/{id} [delete]
func deleteIPRuleHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.IPAC.DeleteRule(c.Request.Context(), c.Param("id")); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// @Summary IP 规则变更广播 reload（全节点）
// @Tags open-ip
// @ID admin.gw.ip-rules.reload
// @Success 200 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/open/ip-reload [post]
func reloadIPHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Engine.Reload(c.Request.Context(), deps.Pool); err != nil {
			writeGwError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}
