package system

import (
	"context"
	"strconv"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/task"
	systemRepo "NetyAdmin/internal/repository/system"
)

type TaskService interface {
	ListTasks(ctx context.Context) ([]map[string]interface{}, error)
	RunTask(ctx context.Context, name string) error
	StopTask(ctx context.Context, name string, operatorID uint) error
	StartTask(ctx context.Context, name string, operatorID uint) error
	ReloadTask(ctx context.Context, name string) error
	UpdateTask(ctx context.Context, name string, enabled bool, spec string, operatorID uint) error
	ListLogs(ctx context.Context, name string, page, size int) ([]*system.TaskLog, int64, error)
}

type taskService struct {
	manager *task.Manager
	logRepo systemRepo.TaskLogRepository
	cfgRepo systemRepo.ConfigRepository
	watcher configsync.ConfigWatcher
}

func NewTaskService(manager *task.Manager, logRepo systemRepo.TaskLogRepository, cfgRepo systemRepo.ConfigRepository, watcher configsync.ConfigWatcher) TaskService {
	s := &taskService{
		manager: manager,
		logRepo: logRepo,
		cfgRepo: cfgRepo,
		watcher: watcher,
	}

	// 注册回调：执行完成后保存日志
	manager.SetOnFinish(func(name string, info task.ExecutionInfo) {
		log := &system.TaskLog{
			Name:      name,
			StartTime: info.StartTime,
			EndTime:   info.EndTime,
			Duration:  info.Duration.Seconds(),
			Status:    info.Status,
			Message:   info.Message,
		}
		// 穿透配置检查：如果任务日志开关关闭，则跳过持久化
		if val, exists := s.watcher.GetConfig("task_config", "log_enabled"); exists && (val == "false" || val == "0") {
			return
		}

		// 使用 Background Context 因为这是异步回调，不应受原始请求控制
		_ = s.logRepo.Create(context.Background(), log)
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
	_ = s.cfgRepo.Upsert(ctx, &system.SysConfig{
		GroupName:   group,
		ConfigKey:   key,
		ConfigValue: "false",
		ValueType:   "boolean",
		Operator:    entity.Operator{UpdatedBy: operatorID},
	})

	// 同步内存中的配置
	_ = s.watcher.ForceReload(ctx)

	return nil
}

func (s *taskService) StartTask(ctx context.Context, name string, operatorID uint) error {
	// 持久化状态为启用
	group := "task_config"
	key := cache.KeyTaskEnabled(name)
	_ = s.cfgRepo.Upsert(ctx, &system.SysConfig{
		GroupName:   group,
		ConfigKey:   key,
		ConfigValue: "true",
		ValueType:   "boolean",
		Operator:    entity.Operator{UpdatedBy: operatorID},
	})

	// 同步内存中的配置
	_ = s.watcher.ForceReload(ctx)

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

	// 更新 enabled
	enabledKey := cache.KeyTaskEnabled(name)
	enabledVal := strconv.FormatBool(enabled)
	if err := s.cfgRepo.Upsert(ctx, &system.SysConfig{
		GroupName:   group,
		ConfigKey:   enabledKey,
		ConfigValue: enabledVal,
		ValueType:   "boolean",
		Operator:    entity.Operator{UpdatedBy: operatorID},
	}); err != nil {
		return err
	}

	// 更新 spec
	specKey := cache.KeyTaskSpec(name)
	if err := s.cfgRepo.Upsert(ctx, &system.SysConfig{
		GroupName:   group,
		ConfigKey:   specKey,
		ConfigValue: spec,
		ValueType:   "string",
		Operator:    entity.Operator{UpdatedBy: operatorID},
	}); err != nil {
		return err
	}

	// 强制重载配置并重启任务
	_ = s.watcher.ForceReload(ctx)
	return s.manager.UpdateTaskSpec(ctx, name, enabled, spec)
}

func (s *taskService) ListLogs(ctx context.Context, name string, page, size int) ([]*system.TaskLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	return s.logRepo.List(ctx, name, page, size)
}
