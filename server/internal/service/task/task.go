package task

import (
	"context"
	"log/slog"
	"strconv"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/domain/entity/system"
	taskEntity "NetyAdmin/internal/domain/entity/task"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/requestid"
	"NetyAdmin/internal/pkg/task"
	systemRepo "NetyAdmin/internal/repository/system"
	taskRepo "NetyAdmin/internal/repository/task"
)

type TaskLogRecordFunc func(ctx context.Context, log *taskEntity.TaskLog) error

type TaskService interface {
	ListTasks(ctx context.Context) ([]map[string]interface{}, error)
	RunTask(ctx context.Context, name string) error
	StopTask(ctx context.Context, name string, operatorID uint) error
	StartTask(ctx context.Context, name string, operatorID uint) error
	ReloadTask(ctx context.Context, name string) error
	UpdateTask(ctx context.Context, name string, enabled bool, spec string, operatorID uint) error
	ListLogs(ctx context.Context, name string, page, size int) ([]*taskEntity.TaskLog, int64, error)
}

type taskService struct {
	manager       *task.Manager
	logRepo       taskRepo.TaskLogRepository
	cfgRepo       systemRepo.ConfigRepository
	watcher       configsync.ConfigWatcher
	logRecordFunc TaskLogRecordFunc
	tm            database.TxManager
}

func NewTaskService(manager *task.Manager, logRepo taskRepo.TaskLogRepository, cfgRepo systemRepo.ConfigRepository, watcher configsync.ConfigWatcher, logRecordFunc TaskLogRecordFunc, tm database.TxManager) TaskService {
	s := &taskService{
		manager:       manager,
		logRepo:       logRepo,
		cfgRepo:       cfgRepo,
		watcher:       watcher,
		logRecordFunc: logRecordFunc,
		tm:            tm,
	}

	manager.SetOnFinish(func(name string, info task.ExecutionInfo) {
		logRecord := &taskEntity.TaskLog{
			Name:      name,
			StartTime: info.StartTime,
			EndTime:   info.EndTime,
			Duration:  info.Duration.Seconds(),
			Status:    info.Status,
			Message:   info.Message,
		}

		if val, exists := s.watcher.GetConfig("task_config", "log_enabled"); exists && (val == "false" || val == "0") {
			return
		}

		ctx := context.Background()
		if info.RequestID != "" {
			ctx = requestid.WithRequestID(ctx, info.RequestID)
		}
		if err := s.logRecordFunc(ctx, logRecord); err != nil {
			slog.Warn("record task log failed", "taskName", name, "err", err)
		}
	})

	return s
}

func (s *taskService) ListTasks(ctx context.Context) ([]map[string]interface{}, error) {
	metas := s.manager.GetTasksStatus()
	states := s.manager.GetRuntimeStates()

	// 从数据库配置中获取覆盖配置 (group: task_config)
	dbConfigs := s.watcher.GetGroupConfigs("task_config")

	var result []map[string]interface{}
	for _, meta := range metas {
		state, exists := states[meta.Name]
		if !exists {
			state = task.RuntimeState{}
		}

		// 检查数据库中是否有覆盖配置
		enabled := meta.Enabled
		if val, ok := dbConfigs["task:"+meta.Name+":enabled"]; ok {
			enabled = (val == "true" || val == "1")
		}

		spec := meta.Spec
		if val, ok := dbConfigs["task:"+meta.Name+":spec"]; ok {
			spec = val
		}

		// 如果内存中没有最后执行记录，尝试从数据库日志中恢复
		lastRunTime := state.LastRunTime
		lastStatus := state.LastStatus
		lastMessage := state.LastMessage
		lastDuration := state.LastDuration.Seconds()

		if lastRunTime.IsZero() || lastRunTime.Year() <= 1 {
			if latestLog, _ := s.logRepo.GetLatest(ctx, meta.Name); latestLog != nil {
				lastRunTime = latestLog.StartTime
				lastStatus = latestLog.Status
				lastMessage = latestLog.Message
				lastDuration = latestLog.Duration
			}
		}

		item := map[string]interface{}{
			"name":           meta.Name,
			"displayName":    meta.DisplayName,
			"type":           meta.Type,
			"spec":           spec,
			"weight":         meta.Weight,
			"enabled":        enabled,
			"isRunning":      state.IsRunning,
			"lastRunTime":    lastRunTime,
			"lastDuration":   lastDuration,
			"lastStatus":     lastStatus,
			"lastMessage":    lastMessage,
			"executionCount": state.ExecutionCount,
		}
		result = append(result, item)
	}

	return result, nil
}

func (s *taskService) RunTask(ctx context.Context, name string) error {
	return s.manager.ManualRun(ctx, name)
}

func (s *taskService) StopTask(ctx context.Context, name string, operatorID uint) error {
	// 先停掉运行中的实例
	if err := s.manager.StopTask(name); err != nil {
		return err
	}

	// 持久化状态为禁用
	group := "task_config"
	key := cache.KeyTaskEnabled(name)
	if err := s.cfgRepo.Upsert(ctx, &system.SysConfig{
		GroupName:   group,
		ConfigKey:   key,
		ConfigValue: "false",
		ValueType:   "boolean",
		Operator:    entity.Operator{UpdatedBy: operatorID},
	}); err != nil {
		slog.Error("upsert task config failed", "key", key, "err", err)
	}

	// 同步内存中的配置
	if err := s.watcher.ForceReload(ctx); err != nil {
		slog.Warn("force reload config failed", "err", err)
	}

	return nil
}

func (s *taskService) StartTask(ctx context.Context, name string, operatorID uint) error {
	// 持久化状态为启用
	group := "task_config"
	key := cache.KeyTaskEnabled(name)
	if err := s.cfgRepo.Upsert(ctx, &system.SysConfig{
		GroupName:   group,
		ConfigKey:   key,
		ConfigValue: "true",
		ValueType:   "boolean",
		Operator:    entity.Operator{UpdatedBy: operatorID},
	}); err != nil {
		slog.Error("upsert task config failed", "key", key, "err", err)
	}

	// 同步内存中的配置
	if err := s.watcher.ForceReload(ctx); err != nil {
		slog.Warn("force reload config failed", "err", err)
	}

	// 在管理器中标记为启用并启动
	// 注意：这里需要先更新管理器内部的 enabled 状态，否则 StartTask 会报错
	var spec string
	if val, ok := s.watcher.GetConfig(group, cache.KeyTaskSpec(name)); ok {
		spec = val
	}
	if spec == "" {
		// 如果 DB 没配置，尝试从管理器获取现有的
		for _, m := range s.manager.GetTasksStatus() {
			if m.Name == name {
				spec = m.Spec
				break
			}
		}
	}

	if err := s.manager.UpdateTaskSpec(ctx, name, true, spec); err != nil {
		return err
	}

	return nil
}

func (s *taskService) ReloadTask(ctx context.Context, name string) error {
	return s.manager.ReloadTask(ctx, name)
}

func (s *taskService) UpdateTask(ctx context.Context, name string, enabled bool, spec string, operatorID uint) error {
	group := "task_config"

	// TM 单事务原子完成「写入 enabled 配置 + 写入 spec 配置」，任一步失败整体回滚（fail-closed）。
	// 避免下次启动时数据不一致（只有 enabled 无 spec）。
	txCtx, tx := s.tm.Begin(ctx)

	// 更新 enabled
	enabledKey := cache.KeyTaskEnabled(name)
	enabledVal := strconv.FormatBool(enabled)
	if err := s.cfgRepo.Upsert(txCtx, &system.SysConfig{
		GroupName:   group,
		ConfigKey:   enabledKey,
		ConfigValue: enabledVal,
		ValueType:   "boolean",
		Operator:    entity.Operator{UpdatedBy: operatorID},
	}); err != nil {
		slog.Error("task update: upsert enabled failed", "key", enabledKey, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "任务更新失败")
	}

	// 更新 spec
	specKey := cache.KeyTaskSpec(name)
	if err := s.cfgRepo.Upsert(txCtx, &system.SysConfig{
		GroupName:   group,
		ConfigKey:   specKey,
		ConfigValue: spec,
		ValueType:   "string",
		Operator:    entity.Operator{UpdatedBy: operatorID},
	}); err != nil {
		slog.Error("task update: upsert spec failed", "key", specKey, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "任务更新失败")
	}

	if err := s.tm.Commit(tx); err != nil {
		slog.Error("task update: commit failed", "err", err)
		return errorx.New(errorx.CodeInternalError, "任务更新失败")
	}

	// Commit 后调用 manager.UpdateTaskSpec（进程内状态，不进事务）
	// 强制重载配置并重启任务
	if err := s.watcher.ForceReload(ctx); err != nil {
		slog.Warn("force reload config failed", "err", err)
	}
	return s.manager.UpdateTaskSpec(ctx, name, enabled, spec)
}

func (s *taskService) ListLogs(ctx context.Context, name string, page, size int) ([]*taskEntity.TaskLog, int64, error) {
	page, size = pagination.NormalizePagination(page, size)
	return s.logRepo.List(ctx, name, page, size)
}
