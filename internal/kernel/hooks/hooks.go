// hooks.go：阶段/类别定义与元数据。
// 四类钩子统一注册表：lifecycle（6 段）/ request（6 段）/ ws（3 段）/ job（3 段）。
// 每阶段元数据：错误策略（fail-fast 首错中止 / aggregate 聚合继续）、默认超时、是否异步。
package hooks

import (
	"fmt"
	"time"
)

// Category 是钩子类别。
type Category string

const (
	CategoryLifecycle Category = "lifecycle"
	CategoryRequest   Category = "request"
	CategoryWS        Category = "ws"
	CategoryJob       Category = "job"
)

// Stage 是阶段标识（category 内唯一；全表唯一由前缀保证）。
type Stage string

// Stages 是全部已登记阶段（按元数据表序，诊断遍历用）。
var Stages = []Stage{
	StageBeforeBootstrap, StageAfterBootstrap, StageBeforeListen, StageAfterListen,
	StageBeforeShutdown, StageAfterShutdown,
	StageRequestReceived, StageRequestAuthenticated, StageBeforeHandler, StageAfterHandler,
	StageResponseSent, StageRequestError,
	StageWSConnect, StageWSMessage, StageWSDisconnect,
	StageJobBeforeRun, StageJobAfterRun, StageJobError,
}

// 生命周期 6 段（§7.2；与 bootstrap.Stage 值一致， 收敛后 bootstrap 经本包别名）。
const (
	StageBeforeBootstrap Stage = "before_bootstrap"
	StageAfterBootstrap  Stage = "after_bootstrap"
	StageBeforeListen    Stage = "before_listen"
	StageAfterListen     Stage = "after_listen"
	StageBeforeShutdown  Stage = "before_shutdown"
	StageAfterShutdown   Stage = "after_shutdown"
)

// 请求 6 段（§7.3）。
const (
	StageRequestReceived      Stage = "request_received"      // 可短路
	StageRequestAuthenticated Stage = "request_authenticated" // 可短路
	StageBeforeHandler        Stage = "before_handler"        // 可短路
	StageAfterHandler         Stage = "after_handler"         // log-only
	StageResponseSent         Stage = "response_sent"         // 异步（§7.5 强制）
	StageRequestError         Stage = "request_error"         // 异步
)

// WS 3 段（§7.4）。
const (
	StageWSConnect    Stage = "ws_connect"    // 可拒绝（ACL）
	StageWSMessage    Stage = "ws_message"    // 可拒绝
	StageWSDisconnect Stage = "ws_disconnect" // 清理
)

// Job 3 段（§7.4）。
const (
	StageJobBeforeRun Stage = "job_before_run" // 可拒绝（跳过本次执行）
	StageJobAfterRun  Stage = "job_after_run"  // 记录
	StageJobError     Stage = "job_error"      // 告警
)

// 错误策略。
type policy int

const (
	policyFailFast  policy = iota // 首错中止链并返回（可短路阶段）
	policyAggregate               // 全部执行，错误聚合返回（log-only/排空段）
)

// stageMeta 是阶段元数据。
type stageMeta struct {
	category Category
	policy   policy
	timeout  time.Duration // 阶段默认（同步默认 2s，§7.5；排空段 5s §7.2）
	async    bool          // 异步触发（ResponseSent/RequestError §7.5）
}

const (
	defaultHookTimeout   = 2 * time.Second
	shutdownStageTimeout = 5 * time.Second
)

// stageMetas 全阶段元数据表（新增阶段必须在此登记）。
var stageMetas = map[Stage]stageMeta{
	// lifecycle（§7.2）
	StageBeforeBootstrap: {CategoryLifecycle, policyFailFast, defaultHookTimeout, false},
	StageAfterBootstrap:  {CategoryLifecycle, policyFailFast, defaultHookTimeout, false},
	StageBeforeListen:    {CategoryLifecycle, policyFailFast, defaultHookTimeout, false},
	StageAfterListen:     {CategoryLifecycle, policyAggregate, defaultHookTimeout, false},
	StageBeforeShutdown:  {CategoryLifecycle, policyAggregate, shutdownStageTimeout, false},
	StageAfterShutdown:   {CategoryLifecycle, policyAggregate, defaultHookTimeout, false},
	// request（§7.3）
	StageRequestReceived:      {CategoryRequest, policyFailFast, defaultHookTimeout, false},
	StageRequestAuthenticated: {CategoryRequest, policyFailFast, defaultHookTimeout, false},
	StageBeforeHandler:        {CategoryRequest, policyFailFast, defaultHookTimeout, false},
	StageAfterHandler:         {CategoryRequest, policyAggregate, defaultHookTimeout, false},
	StageResponseSent:         {CategoryRequest, policyAggregate, defaultHookTimeout, true},
	StageRequestError:         {CategoryRequest, policyAggregate, defaultHookTimeout, true},
	// ws（§7.4）
	StageWSConnect:    {CategoryWS, policyFailFast, defaultHookTimeout, false},
	StageWSMessage:    {CategoryWS, policyFailFast, defaultHookTimeout, false},
	StageWSDisconnect: {CategoryWS, policyAggregate, defaultHookTimeout, false},
	// job（§7.4）
	StageJobBeforeRun: {CategoryJob, policyFailFast, defaultHookTimeout, false},
	StageJobAfterRun:  {CategoryJob, policyAggregate, defaultHookTimeout, false},
	StageJobError:     {CategoryJob, policyAggregate, defaultHookTimeout, true},
}

// meta 取阶段元数据；未登记阶段返回错误（阶段必须显式登记，防拼写静默）。
func meta(stage Stage) (stageMeta, error) {
	m, ok := stageMetas[stage]
	if !ok {
		return stageMeta{}, fmt.Errorf("hooks: unknown stage %q", stage)
	}
	return m, nil
}
