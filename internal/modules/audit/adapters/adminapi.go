// adminapi.go：审计查询 HTTP 端点（V1 log admin 7 端点 RESTful 收敛——
// 列表/详情/删除/批删/CSV 导出 5 端点；security 子面 = type 过滤，§8.5 统一
// 记录模型不另设端点族；error_log 不入 DB（AGENTS §11），无对应端点）。
// 模块自注册模式：cmd 组装经 Module.RegisterAdminRoutes 挂载。
package adapters

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/audit/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/audit/application"
	"github.com/netyilei/netyadmin/internal/modules/audit/domain"
)

// Deps 是 handlers 依赖（module.go 装配注入）。
type Deps struct {
	Query      *application.QueryService
	Authorizer authz.Authorizer // RBAC 判定面（cmd 注入；nil=fail-closed 全拒）
	OnDeny     func(*gin.Context, authn.Principal, string, string)
}

// RegisterAdminRoutes 注册审计查询端点（module.go 调用）。
func RegisterAdminRoutes(rg gin.IRouter, deps Deps) {
	authzMW := transport.AuthzMiddleware(deps.Authorizer, transport.PrincipalFrom, deps.OnDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}

	logs := rg.Group("/operation-logs")
	logs.GET("", secured(keys.PermLogsList, listLogsHandler(deps))...)
	logs.GET("/export", secured(keys.PermLogsExport, exportLogsHandler(deps))...)
	logs.GET("/:id", secured(keys.PermLogsGet, getLogHandler(deps))...)
	// 无删除面：logging.md §3「追加写、不可篡改」——留痕清理只走 RetentionJob。
}

// filterFromQuery 组装列表过滤（type=3 即 security 子面；时间 RFC3339 可选）。
func filterFromQuery(c *gin.Context) domain.QueryFilter {
	f := domain.QueryFilter{
		Action:    c.Query("action"),
		ActorID:   c.Query("actor_id"),
		Result:    c.Query("result"),
		RequestID: c.Query("request_id"),
	}
	if v, err := strconv.Atoi(c.Query("type")); err == nil && v > 0 {
		f.Type = domain.RecordType(v)
	}
	if v, err := time.Parse(time.RFC3339, c.Query("time_from")); err == nil {
		f.TimeFrom = v
	}
	if v, err := time.Parse(time.RFC3339, c.Query("time_to")); err == nil {
		f.TimeTo = v
	}
	return f
}

// @Summary 审计日志列表
// @ID admin.audit.logs.list
// @Description type=1 operation / 2 access / 3 security / 4 system
// @Tags operation-logs
// @Produce json
// @Param type query int false "类型过滤"
// @Param action query string false "动作前缀过滤"
// @Param actor_id query string false "操作者过滤"
// @Param result query string false "结果过滤（success/deny/error）"
// @Param request_id query string false "请求 ID 过滤"
// @Param time_from query string false "起始时间（RFC3339）"
// @Param time_to query string false "截止时间（RFC3339）"
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} transport.Page
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/operation-logs [get]
func listLogsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, size := transport.BindPage(c)
		f := filterFromQuery(c)
		f.Offset = transport.Offset(page, size)
		f.Limit = size
		list, total, err := deps.Query.List(c.Request.Context(), f)
		if err != nil {
			writeAuditError(c, err)
			return
		}
		transport.OK(c, transport.NewPage(list, total, page, size))
	}
}

// @Summary 审计日志详情
// @Tags operation-logs
// @ID admin.logs.get
// @Param id path string true "记录 ID"
// @Success 200 {object} domain.Record
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/operation-logs/{id} [get]
func getLogHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		rec, err := deps.Query.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeAuditError(c, err)
			return
		}
		transport.OK(c, rec)
	}
}

// @Summary 审计日志导出（CSV）
// @ID admin.audit.logs.export
// @Description 同列表过滤条件；超 10000 行拒绝（缩小过滤范围）
// @Tags operation-logs
// @Produce text/csv
// @Param type query int false "类型过滤"
// @Param action query string false "动作前缀过滤"
// @Param actor_id query string false "操作者过滤"
// @Param result query string false "结果过滤"
// @Param time_from query string false "起始时间（RFC3339）"
// @Param time_to query string false "截止时间（RFC3339）"
// @Success 200 {file} file "CSV 附件"
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/operation-logs/export [get]
func exportLogsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := deps.Query.Export(c.Request.Context(), filterFromQuery(c))
		if err != nil {
			writeAuditError(c, err)
			return
		}
		filename := "operation-logs-" + time.Now().UTC().Format("20060102T150405Z") + ".csv"
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
		c.Status(http.StatusOK)
		w := csv.NewWriter(c.Writer)
		_ = w.Write([]string{"id", "type", "actor_type", "actor_id", "action", "resource",
			"result", "biz_code", "ip", "user_agent", "request_id", "trace_id", "node_id", "created_at"})
		for _, r := range rows {
			if err := w.Write([]string{
				csvSafe(r.ID), strconv.Itoa(int(r.Type)), csvSafe(r.ActorType), csvSafe(r.ActorID),
				csvSafe(r.Action), csvSafe(r.Resource), csvSafe(r.Result), csvSafe(r.BizCode),
				csvSafe(r.IP), csvSafe(r.UserAgent), csvSafe(r.RequestID), csvSafe(r.TraceID),
				csvSafe(r.NodeID), r.CreatedAt.Format(time.RFC3339),
			}); err != nil {
				return // 客户端断开——写入失败静默终止（响应流已破）
			}
		}
		w.Flush()
	}
}

// csvSafe 中和 CSV 公式注入（OWASP：=/+/-/@ 制表符开头的内容加 ' 前缀——
// Excel/Libs 公式执行面；audit-m4 数-A3 触发点=导出端点已交付）。
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '	', '':
		return "'" + s
	}
	return s
}

// writeAuditError 查询面错误映射（errorx 信封）：ErrNotFound → 10003；
// ErrExportTooMany → 10001（参数面——过滤过宽）；其余归一。
func writeAuditError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		transport.Fail(c, errorx.New(errorx.CodeNotFound))
	case errors.Is(err, application.ErrExportTooMany):
		transport.Fail(c, errorx.New(errorx.CodeInvalidParams, "reason", "export exceeds cap, narrow filters"))
	default:
		transport.FailOf(c, err)
	}
}
