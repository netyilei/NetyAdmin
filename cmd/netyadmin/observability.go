// observability.go：admin 观测面（指标概览/指标浏览器/阈值展示）。
//
// 数据源=进程内共享 Prometheus registry（/metrics 同一实例）：基座 na_* +
// 模块/下游自注册的 na_<module>_* 全部可见——无需外部 Prometheus/Grafana。
// 阈值评估=请求时计算（展示型告警；不落库/不推送，外部告警走 /metrics）。
package main

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
)

// obsPermView 是观测页权限码（与 rbac_apis 种子对偶——check-rbac-perms）。
const obsPermView = "admin.observability.view"

// 指标浏览器样本上限（单 family；防大标签族把响应撑爆）。
const (
	obsSampleLimitDefault = 200
	obsSampleLimitMax     = 1000
)

// ObservabilityDeps 是观测面依赖（cmd 组合根持有）。
type ObservabilityDeps struct {
	Metrics *observability.Metrics
	// NodeID 是本节点标识（多机部署下页面必须标明“本页是本节点视图”）。
	NodeID string
	// Thresholds 是已合并的阈值表（main 在装配早期由 MergeThresholds 计算并
	// 告警未知键——与 role 无关，core 角色节点也可见配置错误）。
	Thresholds map[string]float64
	// State 返回运行期状态模式（health.ResolveMode；空串=未知）。
	State func() string
}

// sampleWindow 保存上次抓取的总量（窗口差分——QPS 与“突增”错误率）。
type sampleWindow struct {
	mu        sync.Mutex
	lastAt    time.Time
	lastTotal float64
	last5xx   float64
}

// observe 计算窗口速率；首次/无流量/计数器回退（进程重启）=windowed false
// （调用方回退累计口径）。
func (w *sampleWindow) observe(s observability.Summary, now time.Time) (qps, errRate float64, windowed bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	prevAt, prevTotal, prev5xx := w.lastAt, w.lastTotal, w.last5xx
	w.lastAt, w.lastTotal, w.last5xx = now, s.HTTPRequestsTotal, s.HTTP5xxTotal
	if prevAt.IsZero() {
		return 0, 0, false
	}
	elapsed := now.Sub(prevAt).Seconds()
	dTotal := s.HTTPRequestsTotal - prevTotal
	if elapsed <= 0 || dTotal <= 0 {
		return 0, 0, false
	}
	d5xx := s.HTTP5xxTotal - prev5xx
	if d5xx < 0 {
		return 0, 0, false
	}
	return dTotal / elapsed, d5xx / dTotal, true
}

// RegisterObservabilityRoutes 注册观测面端点（authed 组；权限 admin.observability.view）。
func RegisterObservabilityRoutes(authed gin.IRouter, deps ObservabilityDeps,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) {

	thresholds := deps.Thresholds
	if thresholds == nil { // 防御面：组合根必传（nil=仅默认集）
		thresholds, _ = observability.MergeThresholds(nil)
	}
	authzMW := transport.AuthzMiddleware(authorizer, transport.PrincipalFrom, onDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}
	win := &sampleWindow{}
	grp := authed.Group("/system/observability")
	grp.GET("/overview", secured(obsPermView, overviewHandler(deps, thresholds, win))...)
	grp.GET("/metrics", secured(obsPermView, metricsHandler(deps))...)
}

// overviewHandler 指标概览（卡片 + 窗口速率 + 阈值告警）。
// @Summary 观测概览（HTTP/DB/Bus/Scheduler/Realtime/Message 概览 + 阈值告警）
// @Tags observability
// @ID admin.observability.overview
// @Produce json
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /admin/v1/system/observability/overview [get]
func overviewHandler(deps ObservabilityDeps, thresholds map[string]float64, win *sampleWindow) gin.HandlerFunc {
	return func(c *gin.Context) {
		families, err := deps.Metrics.Snapshot()
		if err != nil {
			transport.FailOf(c, errorx.Wrap(errorx.CodeUnavailable, err, "reason", "metrics snapshot"))
			return
		}
		summary := observability.Summarize(families)
		if deps.State != nil {
			summary.StateMode = deps.State()
		}
		qps, windowErrRate, windowed := win.observe(summary, time.Now())

		// 告警用窗口口径（有流量时）；窗口不可用回退累计。
		evalSummary := summary
		if windowed {
			evalSummary.HTTP5xxRate = windowErrRate
		}
		alerts := observability.Evaluate(evalSummary, thresholds)

		transport.OK(c, gin.H{
			"node_id": deps.NodeID,
			"summary": summary,
			"window": gin.H{
				"qps":           qps,
				"http_5xx_rate": windowErrRate,
				"windowed":      windowed,
			},
			"alerts":     alerts,
			"thresholds": thresholds,
		})
	}
}

// metricsHandler 指标浏览器（na_* 全量，含模块/下游自注册；family 前缀过滤）。
// @Summary 指标快照（Prometheus 族投影；含下游自注册 na_*）
// @Tags observability
// @ID admin.observability.metrics
// @Produce json
// @Param family query string false "指标名前缀过滤（如 na_message）"
// @Param limit query int false "单 family 样本上限（默认 200，最大 1000）"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /admin/v1/system/observability/metrics [get]
func metricsHandler(deps ObservabilityDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		families, err := deps.Metrics.Snapshot()
		if err != nil {
			transport.FailOf(c, errorx.Wrap(errorx.CodeUnavailable, err, "reason", "metrics snapshot"))
			return
		}
		prefix := strings.TrimSpace(c.Query("family"))
		limit := obsSampleLimit(c.Query("limit"))
		out := make([]observability.MetricFamily, 0, len(families))
		truncated := false
		for _, fam := range families {
			if prefix != "" && !strings.HasPrefix(fam.Name, prefix) {
				continue
			}
			if len(fam.Samples) > limit {
				fam.Samples = fam.Samples[:limit]
				truncated = true
			}
			out = append(out, fam)
		}
		transport.OK(c, gin.H{
			"families":  out,
			"total":     len(out),
			"truncated": truncated,
		})
	}
}

// obsSampleLimit 解析样本上限（非法/缺省=默认；越界钳制）。
func obsSampleLimit(raw string) int {
	if raw == "" {
		return obsSampleLimitDefault
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return obsSampleLimitDefault
	}
	if n > obsSampleLimitMax {
		return obsSampleLimitMax
	}
	return n
}
