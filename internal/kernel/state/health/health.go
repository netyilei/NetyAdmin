// health.go：就绪聚合器 + 状态模式解析（AGENTS §5、contracts/state-policies.yaml）。
package health

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Mode 是运行期状态模式（contracts degraded_modes 枚举）。
type Mode string

const (
	ModeNormal           Mode = "NORMAL"
	ModeL2Degraded       Mode = "L2_DEGRADED"
	ModeCoordinationDegr Mode = "COORDINATION_DEGRADED"
	ModeSessionDegr      Mode = "SESSION_DEGRADED"
	ModeCacheBypass      Mode = "CACHE_BYPASS"
	ModeBusDegraded      Mode = "BUS_DEGRADED"
)

// Checker 是依赖就绪探针（返回 error = 未就绪）。
type Checker func(ctx context.Context) error

// Aggregator 聚合依赖就绪探针（/health/ready 数据源；并发安全）。
type Aggregator struct {
	mu         sync.RWMutex
	checks     map[string]Checker
	degradable map[string]bool // 失败只降级不翻转就绪（非流量摘除面，如只读副本）
}

// NewAggregator 创建就绪聚合器。
func NewAggregator() *Aggregator {
	return &Aggregator{checks: map[string]Checker{}, degradable: map[string]bool{}}
}

// Register 注册依赖探针（同名覆盖——依赖重连场景）。
func (a *Aggregator) Register(name string, c Checker) {
	if name == "" || c == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checks[name] = c
	delete(a.degradable, name)
}

// RegisterDegradable 注册「降级型」探针：失败在 checks 明细中可见
// （"degraded: ..."），但不翻转整体就绪。
//
// （ready 两态契约）：/health/ready 只有 ok/unready 两态、unready=摘全流量。
// 只读副本故障时写面（主库）仍可用——副本探针若参与整体判定会把可用实例摘出
// LB（放大故障），故以独立 check 名（pg_replica）入明细供监控辨识，聚合不置
// unready。读面降级语义由读消费方（Cluster.Read 使用方）自行回退主库/报错。
func (a *Aggregator) RegisterDegradable(name string, c Checker) {
	if name == "" || c == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checks[name] = c
	a.degradable[name] = true
}

// Unregister 注销探针（依赖下线）。
func (a *Aggregator) Unregister(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.checks, name)
	delete(a.degradable, name)
}

// Ready 汇聚全部探针：全部通过 → (true, nil 错误项)；任一失败 → false + 明细。
// 降级型探针失败仅记 "degraded: ..."（不翻转 ok）；单个探针 panic 隔离（转失败，
// 不击穿健康检查自身）。
func (a *Aggregator) Ready(ctx context.Context) (bool, map[string]string) {
	a.mu.RLock()
	checks := make(map[string]Checker, len(a.checks))
	degradable := make(map[string]bool, len(a.degradable))
	for n, c := range a.checks {
		checks[n] = c
	}
	for n, d := range a.degradable {
		degradable[n] = d
	}
	a.mu.RUnlock()

	details := make(map[string]string, len(checks))
	ok := true
	for name, c := range checks {
		err := safeCheck(ctx, c)
		degraded := degradable[name]
		switch {
		case err == nil:
			details[name] = "ok"
		case errors.Is(err, context.DeadlineExceeded):
			if degraded {
				details[name] = "degraded: timeout"
				continue
			}
			details[name] = "timeout"
			ok = false
		default:
			if degraded {
				details[name] = "degraded: " + err.Error()
				continue
			}
			details[name] = "fail: " + err.Error()
			ok = false
		}
	}
	return ok, details
}

// Names 已注册探针名（排序，诊断）。
func (a *Aggregator) Names() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	names := make([]string, 0, len(a.checks))
	for n := range a.checks {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func safeCheck(ctx context.Context, c Checker) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("checker panic: %v", p)
		}
	}()
	return c(ctx)
}

// ResolveMode 运行期状态模式解析（contracts topology + state.mode 语义）：
//
//	single 拓扑无 Redis → NORMAL（L2 可选）；
//	multi 拓扑依赖 Redis（redis_required: true）——Redis 探针失败：
//	  strict → COORDINATION_DEGRADED（fail-closed 项拒绝）；
//	  auto（仅 dev）→ CACHE_BYPASS（放行 + 旁路）；
//	启动期 fail-fast 校验（multi 缺 L2/生产 auto）在 kernel/config.Validate。
//
// stateMode/hasRedis/redisOK 由调用方注入。
func ResolveMode(topology, stateMode string, hasRedis, redisOK bool) Mode {
	if topology != "multi" {
		return ModeNormal
	}
	if !hasRedis {
		return ModeL2Degraded
	}
	if !redisOK {
		if stateMode == "auto" {
			return ModeCacheBypass
		}
		return ModeCoordinationDegr
	}
	return ModeNormal
}

// Describe 人类可读的模式说明（诊断端点输出）。
func Describe(m Mode) string {
	switch m {
	case ModeNormal:
		return "所有状态平面正常"
	case ModeL2Degraded:
		return "multi 拓扑缺 Redis（L2 不可用；coordination/counter 降级按策略）"
	case ModeCoordinationDegr:
		return "Redis 探活失败（锁/限流/幂等按策略降级，fail-closed 项拒绝）"
	case ModeCacheBypass:
		return "缓存旁路（全部直读 DB）"
	case ModeSessionDegr:
		return "会话平面降级（token 校验本地 JWT 不受影响；吊销/轮换 fail-closed）"
	case ModeBusDegraded:
		return "总线降级"
	default:
		return strings.TrimSpace("未知模式: " + string(m))
	}
}
