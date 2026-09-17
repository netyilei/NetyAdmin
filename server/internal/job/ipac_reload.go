package job

import (
	"context"
	"encoding/json"
	"log/slog"

	"NetyAdmin/internal/pkg/task"
	ipacService "NetyAdmin/internal/service/ipac"
)

// IPACReloadJob IPAC 规则过期感知重载任务。
//
// 背景：CheckIP 只查内存 trie，不校验规则过期时间；规则集重建仅在启动与
// CRUD/pubsub 通知时触发。无本任务时，已过期的封禁/放行规则会一直驻留
// 内存生效，直到下一次任意 IPAC 变更才被清理。
//
// ReloadCache 内部有指纹 diff：规则无变化时（含未过期的稳定状态）直接跳过
// 重建，本任务以 1 分钟周期轮询的代价可忽略；有过期规则被 GetAllEffective
// 过滤掉时指纹变化触发重建，过期规则在分钟级窗口内失效。
type IPACReloadJob struct {
	svc ipacService.IPACService
}

func NewIPACReloadJob(svc ipacService.IPACService) *IPACReloadJob {
	return &IPACReloadJob{svc: svc}
}

func (j *IPACReloadJob) Name() string {
	return "ipac_reload"
}

func (j *IPACReloadJob) DisplayName() string {
	return "IPAC Expiry Refresher"
}

func (j *IPACReloadJob) Run(ctx context.Context) error {
	if err := j.svc.ReloadCache(ctx); err != nil {
		slog.Error("IPAC reload job failed", "err", err)
		return err
	}
	return nil
}

func (j *IPACReloadJob) Execute(ctx context.Context, payload json.RawMessage) error {
	return j.Run(ctx)
}

// DefaultMetadata 默认间隔执行（运维级，权重低于业务任务）。
func (j *IPACReloadJob) DefaultMetadata() task.TaskMetadata {
	return task.TaskMetadata{
		Name:        j.Name(),
		DisplayName: j.DisplayName(),
		Type:        task.TypeInterval,
		Spec:        "1m",
		Weight:      task.WeightLow,
		Enabled:     true,
	}
}
