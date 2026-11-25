// operations.go：运维管理端点（scheduler 任务中心 + Bus 死信重放）。
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/scheduler"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	ckeys "github.com/netyilei/netyadmin/internal/modules/configcenter/api/keys"
	ccapplication "github.com/netyilei/netyadmin/internal/modules/configcenter/application"
	ccdomain "github.com/netyilei/netyadmin/internal/modules/configcenter/domain"
)

// —— 任务中心（scheduler 管理面）——
// 任务中心管理端点（V1 task 7 收敛——scheduler 管理面
// 包装：任务目录/运行记录/触发/暂停/恢复）。scheduler 内核面交付
// Catalog/TriggerNow/Pause/Resume/RecentRuns；本文件职责：
//   - 纯转发：jobs/runs（kernel 快照/记录面）。
//   - F2 根修：pause/resume 经 configcenter 键外置（集群一致+重启恢复）；
//     trigger 如实上报确定性守卫结果（paused/not-leader 返 triggered:false）。

// envelopeSwaggerAlias 是 swag 注解的 import 使用锚点（cmd 包不直接消费
// errorx 信封，但注解 {object} errorx.Envelope 需当前文件 import 可解析——
// taskcenter 注解补齐）。
//
//lint:ignore U1000 swag import 锚点（errorx.Envelope 注解引用需当前文件 import 可解析）
type envelopeSwaggerAlias = errorx.Envelope

// taskcenterPerm 是权限码（act 与 rbac_apis 种子同源——12566）。
const (
	taskPermView    = "admin.taskcenter.view"
	taskPermOperate = "admin.taskcenter.operate"
)

// RegisterTaskcenterRoutes 注册任务中心管理端点（cmd 组合根调用——sched/ccSvc
// 由 main 持有；挂 authed 组，admin.taskcenter.* 权限码 12566 种子）。
func RegisterTaskcenterRoutes(authed gin.IRouter, sched *scheduler.Scheduler,
	ccSvc *ccapplication.Service, authorizer authz.Authorizer,
	onDeny func(*gin.Context, authn.Principal, string, string)) {

	authzMW := transport.AuthzMiddleware(authorizer, transport.PrincipalFrom, onDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}
	tc := authed.Group("/taskcenter")
	tc.GET("/jobs", secured(taskPermView, jobsHandler(sched))...)
	tc.GET("/jobs/:name/runs", secured(taskPermView, runsHandler(sched))...)
	tc.POST("/jobs/:name/trigger", secured(taskPermOperate, triggerHandler(sched))...)
	tc.POST("/jobs/:name/pause", secured(taskPermOperate, pauseHandler(sched, ccSvc))...)
	tc.POST("/jobs/:name/resume", secured(taskPermOperate, resumeHandler(sched, ccSvc))...)
}

// jobsHandler 任务目录快照（status/spec 面——paused/running 状态含）。
// @Summary 任务目录快照（status/spec 面——paused/running 状态含）
// @Tags taskcenter
// @ID admin.taskcenter.jobs
// @Produce json
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/taskcenter/jobs [get]
func jobsHandler(sched *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		transport.OK(c, gin.H{"jobs": sched.Catalog()})
	}
}

// runsHandler 运行记录（logs 面——RecentRuns 按 job 过滤）。
// @Summary 任务运行记录（RecentRuns 按 job 过滤）
// @Tags taskcenter
// @ID admin.taskcenter.runs
// @Produce json
// @Param name path string true "任务名"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/taskcenter/jobs/{name}/runs [get]
func runsHandler(sched *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 50
		runs, err := sched.RecentRuns(c.Request.Context(), c.Param("name"), limit)
		if err != nil {
			transport.FailOf(c, err)
			return
		}
		transport.OK(c, gin.H{"runs": runs})
	}
}

// triggerHandler 立即触发（reload 语义对偶——手动执行一次）。确定性守卫
// （暂停/cluster 非 leader）如实返回 triggered:false+reason（F2——不再假报
// 成功）；overlap 等时序类守卫仍走异步管线（结果不可预判，运行记录可查）。
// @Summary 立即触发（reload 语义对偶——手动执行一次；暂停/非 leader 如实返回 triggered:false+reason）
// @Tags taskcenter
// @ID admin.taskcenter.trigger
// @Produce json
// @Param name path string true "任务名"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/taskcenter/jobs/{name}/trigger [post]
func triggerHandler(sched *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if err := sched.TriggerNow(c.Request.Context(), name); err != nil {
			switch {
			case errors.Is(err, scheduler.ErrNotLeader):
				transport.OK(c, gin.H{"triggered": false, "job": name, "reason": "not-leader"})
			case errors.Is(err, scheduler.ErrPaused):
				transport.OK(c, gin.H{"triggered": false, "job": name, "reason": "paused"})
			default:
				transport.FailOf(c, err)
			}
			return
		}
		transport.OK(c, gin.H{"triggered": true, "job": name})
	}
}

// pauseHandler 暂停（stop 语义对偶）——暂停态外置（persistPausedState）。
// @Summary 暂停任务（stop 语义对偶——暂停态外置 configcenter 持久化）
// @Tags taskcenter
// @ID admin.taskcenter.pause
// @Produce json
// @Param name path string true "任务名"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/taskcenter/jobs/{name}/pause [post]
func pauseHandler(sched *scheduler.Scheduler, ccSvc *ccapplication.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := persistPausedState(c.Request.Context(), ccSvc, sched,
			c.Param("name"), true, transport.PrincipalFrom(c).ID); err != nil {
			transport.FailOf(c, err)
			return
		}
		transport.OK(c, gin.H{"paused": true, "job": c.Param("name")})
	}
}

// resumeHandler 恢复（start 语义对偶）——暂停态外置（persistPausedState）。
// @Summary 恢复任务（start 语义对偶——暂停态外置 configcenter 持久化）
// @Tags taskcenter
// @ID admin.taskcenter.resume
// @Produce json
// @Param name path string true "任务名"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/taskcenter/jobs/{name}/resume [post]
func resumeHandler(sched *scheduler.Scheduler, ccSvc *ccapplication.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := persistPausedState(c.Request.Context(), ccSvc, sched,
			c.Param("name"), false, transport.PrincipalFrom(c).ID); err != nil {
			transport.FailOf(c, err)
			return
		}
		transport.OK(c, gin.H{"resumed": true, "job": c.Param("name")})
	}
}

// persistPausedState 暂停态外置（F2 根修）：先写 configcenter 键（组
// taskcenter、键=job 名、bool 值）——变更面走既有管线（校验+history+commit 后
// Bus 广播 platform.config.changed），各节点 watcher reload 回调应用
// SetPaused（wireTaskcenterPause），重启经启动初始装载恢复；再本地即时应用
// （回调幂等，免本节点等广播）。任一节点 configcenter 未挂接（no-DB 形态）
// =操作失败如实上报，不静默退化为单节点内存态（那正是 F2 缺陷本身）。
func persistPausedState(ctx context.Context, ccSvc *ccapplication.Service,
	sched *scheduler.Scheduler, name string, paused bool, operator string) error {

	if !sched.Has(name) {
		return fmt.Errorf("%w: %s", scheduler.ErrJobNotFound, name)
	}
	val, reason := "false", "taskcenter resume"
	if paused {
		val, reason = "true", "taskcenter pause"
	}
	cfg := ccdomain.Config{
		GroupName: ckeys.GroupTaskcenter, ConfigKey: name,
		ConfigValue: val, ValueType: ccdomain.TypeBool,
		Description: "taskcenter 任务暂停态（pause/resume 写入；删除键=恢复）",
	}
	if err := ccSvc.Create(ctx, cfg, operator, reason); err != nil {
		if !errors.Is(err, ccdomain.ErrKeyExists) {
			return err
		}
		if err := ccSvc.Update(ctx, cfg, operator, reason); err != nil {
			return err
		}
	}
	if paused {
		return sched.Pause(name)
	}
	return sched.Resume(name)
}

// wireTaskcenterPause 接线暂停态消费（F2）：configcenter reload（变更广播含
// 本节点 + 启动初始装载）后按快照 taskcenter 组应用各任务暂停态——集群一致；
// 键缺席=未暂停（configcenter 删除键即恢复——状态面单一语义）；未注册任务
// （模块停用）跳过并告警。须在 scheduler Register 之后、DB 装载（AttachDB 的
// 初始 Reload）之前接线——main 组合根顺序保证。
func wireTaskcenterPause(w *ccapplication.Watcher, sched *scheduler.Scheduler, logMgr *log.Manager) {
	logger := logMgr.Logger("taskcenter", "app")
	w.OnChange(func() {
		declared := map[string]bool{}
		for skey, cfg := range w.Snapshot() {
			if cfg.GroupName == ckeys.GroupTaskcenter {
				declared[skey[len(ckeys.GroupTaskcenter)+1:]] = cfg.ConfigValue == "true"
			}
		}
		for _, info := range sched.Catalog() {
			paused := declared[info.Name] // 键缺席=false
			if paused == info.Paused {
				continue
			}
			apply := sched.Resume
			if paused {
				apply = sched.Pause
			}
			if err := apply(info.Name); err != nil {
				logger.Warn("taskcenter paused state apply skip", "job", info.Name, "err", err)
			}
		}
	})
}

// —— Bus 死信重放 ——
// Bus 死信运维面——DLQ 重放
// 暴露为管理端点（kernel bus 已导出 ReplayDLQ，这里只做 HTTP 包装 + 错误映射）。
//
// 语义：bus_dlq 行按 id 重建信封以 Cluster 域重发布，成功后删除该行（重放后
// 再次失败会经正常投递链重新入 DLQ）；毒行（kind 空=原文留存不可重放）拒绝并
// 提示人工取数。挂 taskcenter 组（运维面同组；权限码复用 admin.taskcenter.operate）。

// dlqReplayRequest 是重放请求体（id=bus_dlq.id）。
type dlqReplayRequest struct {
	ID int64 `json:"id" binding:"required"`
}

// RegisterBusDLQRoutes 注册 DLQ 重放端点（cmd 组合根调用——busInst 由 main 持有）。
func RegisterBusDLQRoutes(authed gin.IRouter, busInst *bus.Bus, authorizer authz.Authorizer,
	onDeny func(*gin.Context, authn.Principal, string, string)) {

	authzMW := transport.AuthzMiddleware(authorizer, transport.PrincipalFrom, onDeny)
	tc := authed.Group("/taskcenter")
	tc.POST("/dlq/replay",
		transport.Secured(authzMW, taskPermOperate, dlqReplayHandler(busInst))...)
}

// dlqReplayHandler 死信重放（bus.ReplayDLQ 包装）。
// @Summary 死信重放（按 bus_dlq.id 重建信封重发布；成功后删除该行）
// @Tags taskcenter
// @ID admin.taskcenter.dlq_replay
// @Accept json
// @Produce json
// @Param body body dlqReplayRequest true "死信行 id"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /admin/v1/taskcenter/dlq/replay [post]
func dlqReplayHandler(busInst *bus.Bus) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dlqReplayRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 {
			transport.FailOf(c, errorx.New(errorx.CodeInvalidParams, "reason", "dlq id"))
			return
		}
		if busInst == nil {
			transport.FailOf(c, errorx.New(errorx.CodeUnavailable, "reason", "bus not wired"))
			return
		}
		err := busInst.ReplayDLQ(c.Request.Context(), req.ID)
		switch {
		case err == nil:
			transport.OK(c, gin.H{"replayed": true, "id": req.ID})
		case errors.Is(err, bus.ErrNotFound):
			transport.FailOf(c, errorx.New(errorx.CodeNotFound, "reason", "dlq row not found"))
		case errors.Is(err, bus.ErrPoisonRow):
			// 毒行（kind 空=原文不可重放）：需人工取数后改 kind 再重放
			transport.FailOf(c, errorx.New(errorx.CodeConflict, "reason", "poison dlq row (manual inspection required)"))
		case errors.Is(err, bus.ErrNoDB):
			transport.FailOf(c, errorx.New(errorx.CodeUnavailable, "reason", "bus dlq requires database"))
		default:
			transport.FailOf(c, err)
		}
	}
}
