package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/recovery"
	"NetyAdmin/internal/pkg/requestid"
	"NetyAdmin/internal/pkg/slogutil"
)

// taskLockReleaseScript 原子比对 token 后删除锁，避免误删其他实例的锁。
// KEYS[1] = lockKey, ARGV[1] = lockToken
var taskLockReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`)

// taskLockRenewScript 原子比对 token 后续期，确保仅锁持有者能续期。
// KEYS[1] = lockKey, ARGV[1] = lockToken, ARGV[2] = TTL seconds
var taskLockRenewScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("EXPIRE", KEYS[1], ARGV[2])
else
	return 0
end
`)

// Manager 任务调度引擎

type Manager struct {
	cfg      *config.TaskConfig
	redisCfg *config.RedisConfig // 引入 Redis 配置用于判断是否启用分布式锁
	redis    *redis.Client       // Redis 客户端实例
	tasks    map[string]Task     // 使用 Map 确保任务唯一性
	cron     *cron.Cron
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.RWMutex
	states   map[string]*RuntimeState
	onFinish func(name string, info ExecutionInfo)
	queue    Queue // 任务分发队列

	// 增强部分：用于动态管理
	cronIDs   map[string]cron.EntryID  // 记录 Cron 任务 ID
	intervals map[string]chan struct{} // 记录 Interval 任务停止通道 (按任务名隔离)
	cancel    context.CancelFunc       // 用于停止所有 Worker
	stopOnce  sync.Once                // 保护 stopChan 的 close 操作，避免 double-close panic
}

// NewManager 创建调度引擎
func NewManager(cfg *config.TaskConfig, redisCfg *config.RedisConfig, redisCli *redis.Client) *Manager {
	m := &Manager{
		cfg:       cfg,
		redisCfg:  redisCfg,
		redis:     redisCli,
		tasks:     make(map[string]Task),
		states:    make(map[string]*RuntimeState),
		cronIDs:   make(map[string]cron.EntryID),
		intervals: make(map[string]chan struct{}),
		cron:      cron.New(cron.WithSeconds()), // 支持到秒级
		stopChan:  make(chan struct{}),
	}

	// 初始化队列驱动
	if redisCfg != nil && redisCfg.Enabled && redisCli != nil {
		m.queue = NewRedisQueue(redisCli, redisCfg.Prefix)
		slog.Info("任务引擎已启用 Redis 分布式队列驱动")
	} else {
		m.queue = NewLocalQueue(1000)
		slog.Info("任务引擎已启用本地 Channel 队列驱动")
	}

	return m
}

// SetOnFinish 设置任务执行完成后的回调（可用于持久化日志）
func (m *Manager) SetOnFinish(fn func(name string, info ExecutionInfo)) {
	m.onFinish = fn
}

// Register 注册一个或多个任务。如果任务名重复，后者将覆盖前者。
func (m *Manager) Register(tasks ...Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range tasks {
		name := t.Name()
		if _, exists := m.tasks[name]; exists {
			slog.Warn("任务重复注册，将使用最新实例", "name", name)
		}
		m.tasks[name] = t
	}
}

// Start 启动调度引擎
func (m *Manager) Start(ctx context.Context) {
	if m.cfg == nil || !m.cfg.Enabled {
		slog.Info("任务引擎未启用，跳过启动")
		return
	}

	// 初始化 Worker 控制上下文
	workerCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	// 1. 获取所有已启用的任务并进行配置合并
	type taskWithConfig struct {
		task     Task
		metadata TaskMetadata
	}
	var enabledTasks []taskWithConfig

	m.mu.RLock()
	for _, t := range m.tasks {
		meta := m.getTaskMetadata(t)
		if meta.Enabled {
			enabledTasks = append(enabledTasks, taskWithConfig{t, meta})
		}
	}
	m.mu.RUnlock()

	// 2. 按权重降序排序（确保 Once 任务按优先级执行）
	sort.Slice(enabledTasks, func(i, j int) bool {
		return enabledTasks[i].metadata.Weight > enabledTasks[j].metadata.Weight
	})

	slog.Info("任务引擎启动中", "count", len(enabledTasks))

	// 3. 分类处理
	for _, tc := range enabledTasks {
		switch tc.metadata.Type {
		case TypeOnce:
			// Once 任务同步顺序执行 (生产者级别：必须等待系统任务完成)
			slog.Info("任务引擎执行同步启动任务", "name", tc.metadata.Name, "weight", tc.metadata.Weight)
			if err := tc.task.Run(ctx); err != nil {
				slog.Error("启动任务执行失败", "name", tc.metadata.Name, "error", err)
			}
		case TypeInterval:
			m.wg.Add(1)
			stopChan := make(chan struct{})
			m.mu.Lock()
			m.intervals[tc.metadata.Name] = stopChan
			m.mu.Unlock()
			// 异步执行间隔任务（GoSafe 包裹 recover + Sentry 上报，防止 panic 导致任务静默退出）
			// runIntervalTask 内部 defer m.wg.Done() 在 panic 时仍会触发
			recovery.GoSafe("task:interval", func() {
				m.runIntervalTask(ctx, tc.task, tc.metadata, stopChan)
			})
		case TypeCron:
			m.registerCronTask(ctx, tc.task, tc.metadata)
		}
	}

	m.cron.Start()

	// 4. 启动后台消费者 Worker
	m.startWorkers(workerCtx)
}

// Dispatch 投递子任务 (实现 Dispatcher 接口)
//
// Task 8.6: 从 ctx 提取 request_id 写入 msg.RequestID，跨 goroutine 传播；
// Worker 在 executePayload 中通过 requestid.WithRequestID 恢复到子 ctx，
// 让任务执行期间的 slog / Sentry 上报能关联到原始触发请求。
func (m *Manager) Dispatch(ctx context.Context, taskName string, payload interface{}, weight int) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload failed: %w", err)
	}

	msg := &Message{
		TaskName:  taskName,
		Payload:   data,
		RequestID: requestid.FromContext(ctx),
	}

	if err := m.queue.Push(ctx, msg, weight); err != nil {
		return fmt.Errorf("push message to queue failed: %w", err)
	}

	return nil
}

func (m *Manager) startWorkers(ctx context.Context) {
	workerCount := m.cfg.Workers
	if workerCount <= 0 {
		workerCount = 5
	}
	slog.Info("任务引擎启动后台 Worker 处理队列任务", "count", workerCount)

	for i := 0; i < workerCount; i++ {
		m.wg.Add(1)
		workerID := i
		// GoSafe 包裹 recover + Sentry 上报，防止单个任务 panic 导致 Worker 静默退出。
		// defer m.wg.Done() 在 fn 内部，panic 时仍会触发（defer 在 recover 捕获前执行）。
		recovery.GoSafe("task:worker", func() {
			defer m.wg.Done()
			for {
				// 改造说明（P2-5）：
				// 原实现把 m.queue.Pop 放在 select 的 default 分支里，
				// 当 ctx/stopChan 都无信号、且 Pop 立即返回（如 LocalQueue 100ms 超时返回 nil）时，
				// default 会被立即命中形成空轮询，极端情况下 CPU 100%。
				// 现改为：先用非阻塞 select 检查停止信号，再在 select 之外调用 Pop。
				// Pop 内部自带阻塞语义（RedisQueue BRPop 5s、LocalQueue select+100ms time.After，
				// 且都监听 ctx.Done），无消息时挂起 Worker，不会自旋。
				select {
				case <-ctx.Done():
					return
				case <-m.stopChan:
					return
				default:
				}

				msg, err := m.queue.Pop(ctx)
				if err != nil {
					// ctx 取消时 Pop 可能返回 ctx.Err，属正常退出路径，不当作错误记录
					if ctx.Err() != nil {
						return
					}
					slog.Error("任务引擎 Worker Pop 消息失败", "worker", workerID, "error", err)
					time.Sleep(time.Second) // 发生错误稍后重试
					continue
				}
				if msg == nil {
					continue // 超时无消息
				}

				m.executePayload(ctx, msg)
			}
		})
	}
}

func (m *Manager) executePayload(ctx context.Context, msg *Message) {
	// Task 8.6: 从 msg.RequestID 恢复 request_id 到子 ctx，
	// 让 t.Execute 内部的 slog / Sentry 上报能关联到原始触发请求。
	if msg.RequestID != "" {
		ctx = requestid.WithRequestID(ctx, msg.RequestID)
	}
	// Task 8.3: 用 slogutil.LoggerFromContext 替代裸 slog，自动携带 request_id 字段。
	logger := slogutil.LoggerFromContext(ctx)

	m.mu.RLock()
	t, exists := m.tasks[msg.TaskName]
	m.mu.RUnlock()

	if !exists {
		logger.Error("任务引擎消费者执行失败: 任务未注册", "name", msg.TaskName)
		return
	}

	// 消费者执行不需要分布式锁，因为队列 Pop 已经是原子操作
	info := ExecutionInfo{
		StartTime: time.Now(),
	}

	// 更新状态为运行中
	m.mu.Lock()
	state, exists := m.states[msg.TaskName]
	if !exists {
		state = &RuntimeState{}
		m.states[msg.TaskName] = state
	}
	state.IsRunning = true
	m.mu.Unlock()

	err := t.Execute(ctx, msg.Payload)

	// 更新状态结束
	info.EndTime = time.Now()
	info.Duration = info.EndTime.Sub(info.StartTime)
	info.Status = "success"
	if err != nil {
		info.Status = "error"
		info.Message = err.Error()
		logger.Error("任务载荷执行失败", "name", msg.TaskName, "error", err)
	}

	m.mu.Lock()
	state.IsRunning = false
	state.LastRunTime = info.StartTime
	state.LastDuration = info.Duration
	state.LastStatus = info.Status
	state.LastMessage = info.Message
	state.ExecutionCount++
	m.mu.Unlock()

	if m.onFinish != nil {
		info.RequestID = msg.RequestID
		m.onFinish(msg.TaskName, info)
	}
}

// StartTask 启动单个任务
func (m *Manager) StartTask(ctx context.Context, name string) error {
	m.mu.Lock()
	t, exists := m.tasks[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("任务 [%s] 不存在", name)
	}
	meta := m.getTaskMetadata(t)
	m.mu.Unlock()

	if !meta.Enabled {
		return fmt.Errorf("任务 [%s] 已被禁用", name)
	}

	// 检查是否已经在运行
	m.mu.RLock()
	state, stateExists := m.states[name]
	if stateExists && state.IsRunning && meta.Type == TypeInterval {
		m.mu.RUnlock()
		return fmt.Errorf("任务 [%s] 正在运行中", name)
	}
	m.mu.RUnlock()

	switch meta.Type {
	case TypeOnce:
		// 异步执行单次任务（GoSafe 包裹 recover + Sentry 上报，防止 panic 影响调度引擎）
		recovery.GoSafe("task:once", func() {
			m.execute(ctx, t)
		})
	case TypeInterval:
		m.wg.Add(1)
		stopChan := make(chan struct{})
		m.mu.Lock()
		m.intervals[name] = stopChan
		m.mu.Unlock()
		// 异步执行间隔任务（GoSafe 包裹 recover + Sentry 上报，防止 panic 导致任务静默退出）
		// runIntervalTask 内部 defer m.wg.Done() 在 panic 时仍会触发
		recovery.GoSafe("task:interval", func() {
			m.runIntervalTask(ctx, t, meta, stopChan)
		})
	case TypeCron:
		m.registerCronTask(ctx, t, meta)
	}

	return nil
}

// StopTask 停止单个任务 (仅针对 Interval 和 Cron 类型)
func (m *Manager) StopTask(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 处理 Interval 任务
	if stopChan, exists := m.intervals[name]; exists {
		close(stopChan)
		delete(m.intervals, name)
		slog.Info("任务引擎间隔任务已发出停止信号", "name", name)
		return nil
	}

	// 2. 处理 Cron 任务
	if entryID, exists := m.cronIDs[name]; exists {
		m.cron.Remove(entryID)
		delete(m.cronIDs, name)
		slog.Info("任务引擎定时任务已从调度器移除", "name", name)
		return nil
	}

	return fmt.Errorf("任务 [%s] 未在运行或不可停止", name)
}

// UpdateTaskSpec 更新任务配置并重启
func (m *Manager) UpdateTaskSpec(ctx context.Context, name string, enabled bool, spec string) error {
	m.mu.Lock()
	_, exists := m.tasks[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("任务 [%s] 不存在", name)
	}

	// 找到对应的 Config 并更新 (内存中)
	if jobCfg, ok := m.cfg.Jobs[name]; ok {
		jobCfg.Enabled = &enabled
		jobCfg.Spec = &spec
		m.cfg.Jobs[name] = jobCfg
	} else {
		// 如果不存在，创建一个
		m.cfg.Jobs[name] = config.JobConfig{
			Enabled: &enabled,
			Spec:    &spec,
		}
	}
	m.mu.Unlock()

	return m.ReloadTask(ctx, name)
}

// ReloadTask 根据当前配置重启单个任务
func (m *Manager) ReloadTask(ctx context.Context, name string) error {
	// 1. 先尝试停止 (无论当前配置如何)
	// StopTask 失败仅 Warn：reload 会继续尝试 StartTask，残留旧实例由 cron 调度去重兜底。
	if err := m.StopTask(name); err != nil {
		slog.Warn("ReloadTask: StopTask failed (will attempt StartTask anyway)",
			"name", name, "error", err)
	}

	// 2. 获取最新元数据
	m.mu.RLock()
	t, exists := m.tasks[name]
	if !exists {
		m.mu.RUnlock()
		return fmt.Errorf("任务 [%s] 不存在", name)
	}
	meta := m.getTaskMetadata(t)
	m.mu.RUnlock()

	// 3. 如果启用，则启动
	if meta.Enabled {
		return m.StartTask(ctx, name)
	}

	slog.Info("任务已处于禁用状态，无需启动", "name", name)
	return nil
}

// ManualRun 手动触发任务执行
func (m *Manager) ManualRun(ctx context.Context, name string) error {
	m.mu.RLock()
	t, exists := m.tasks[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("任务 [%s] 不存在", name)
	}

	m.wg.Add(1)
	// GoSafe 包裹 recover + Sentry 上报，防止手动触发的任务 panic 影响调度引擎。
	// defer m.wg.Done() 在 fn 内部，panic 时仍会触发（defer 在 recover 捕获前执行）。
	recovery.GoSafe("task:manual", func() {
		defer m.wg.Done()
		m.execute(ctx, t)
	})

	return nil
}

// GetTasksStatus 获取所有任务的当前状态 (按权重降序、名称升序排列)
func (m *Manager) GetTasksStatus() []TaskMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []TaskMetadata
	for _, t := range m.tasks {
		meta := m.getTaskMetadata(t)
		list = append(list, meta)
	}

	// 稳定排序：权重降序 -> 名称升序
	sort.Slice(list, func(i, j int) bool {
		if list[i].Weight != list[j].Weight {
			return list[i].Weight > list[j].Weight
		}
		return list[i].Name < list[j].Name
	})

	return list
}

// GetRuntimeStates 获取所有任务的运行状态
func (m *Manager) GetRuntimeStates() map[string]RuntimeState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make(map[string]RuntimeState)
	for name, state := range m.states {
		res[name] = *state
	}
	return res
}

func (m *Manager) execute(ctx context.Context, t Task) {
	name := t.Name()

	// 1. 更新状态：开始执行
	m.mu.Lock()
	state, exists := m.states[name]
	if !exists {
		state = &RuntimeState{}
		m.states[name] = state
	}
	state.IsRunning = true
	m.mu.Unlock()

	info := ExecutionInfo{
		StartTime: time.Now(),
	}

	// 2. 尝试抢占分布式锁 (仅在 Redis 启用时)
	if m.redisCfg != nil && m.redisCfg.Enabled && m.redis != nil {
		lockKey := cache.KeyTaskLock(m.redisCfg.Prefix, name)
		lockToken := uuid.NewString()
		lockTTL := 1 * time.Hour
		// resetRunningState 把 state.IsRunning 改回 false（抢锁失败/出错时回滚）。
		resetRunningState := func() {
			m.mu.Lock()
			state.IsRunning = false
			m.mu.Unlock()
		}

		err := m.redis.SetArgs(ctx, lockKey, lockToken, redis.SetArgs{
			Mode: "NX",
			TTL:  lockTTL,
		}).Err()
		if err != nil {
			// redis.Nil = 未抢到锁（其他实例执行中）；其他 err = 抢锁出错
			// 两种情况都安全起见不执行本实例任务
			if err != redis.Nil {
				slog.Error("任务尝试获取分布式锁失败", "name", name, "error", err)
			} else {
				slog.Info("任务在其他实例中执行，本实例跳过", "name", name)
			}
			resetRunningState()
			return
		}

		// 启动看门狗续期 goroutine：每 TTL/3 续期一次，确保长时间任务不会因锁过期被其他实例抢占。
		// 续期用 Lua 脚本原子比对 token，仅锁持有者能续期。
		watchdogDone := make(chan struct{})
		recovery.GoSafe("task:watchdog", func() {
			ticker := time.NewTicker(lockTTL / 3)
			defer ticker.Stop()
			for {
				select {
				case <-watchdogDone:
					return
				case <-ticker.C:
					result, err := taskLockRenewScript.Run(ctx, m.redis,
						[]string{lockKey}, lockToken, int(lockTTL.Seconds())).Int()
					if err != nil {
						slog.Warn("任务锁续期失败", "name", name, "error", err)
						continue
					}
					if result == 0 {
						// 锁已不属于本实例（TTL 过期后被其他实例抢占）
						slog.Warn("任务锁续期失败: 锁已不属于本实例", "name", name)
						return
					}
				}
			}
		})

		// 执行完毕后释放锁
		// 用 Lua 脚本原子比对 token 后删除，避免误删其他实例的锁。
		// 释放失败仅 Warn：锁自带 TTL 兜底，看门狗已停止后续期，TTL 到期后自动过期。
		defer func() {
			close(watchdogDone) // 停止看门狗
			result, err := taskLockReleaseScript.Run(ctx, m.redis,
				[]string{lockKey}, lockToken).Int()
			if err != nil {
				slog.Warn("task: release distributed lock failed (will auto-expire via TTL)",
					"name", name, "lockKey", lockKey, "error", err)
			} else if result == 0 {
				// 锁已不属于本实例（TTL 过期后被其他实例抢占并释放）
				slog.Warn("task: lock already expired or taken by another instance",
					"name", name, "lockKey", lockKey)
			}
		}()
	}

	// 3. 执行任务
	err := t.Run(ctx)

	// 3. 更新状态：执行结束
	info.EndTime = time.Now()
	info.Duration = info.EndTime.Sub(info.StartTime)
	info.Status = "success"
	if err != nil {
		info.Status = "error"
		info.Message = err.Error()
	}

	m.mu.Lock()
	state.IsRunning = false
	state.LastRunTime = info.StartTime
	state.LastDuration = info.Duration
	state.LastStatus = info.Status
	state.LastMessage = info.Message
	state.ExecutionCount++
	m.mu.Unlock()

	// 4. 回调处理
	if m.onFinish != nil {
		info.RequestID = requestid.FromContext(ctx)
		m.onFinish(name, info)
	}
}

// Stop 停止引擎 (优雅停机)
func (m *Manager) Stop() {
	if m.cfg == nil || !m.cfg.Enabled {
		return
	}
	slog.Info("任务引擎正在发出停止信号...")
	m.cron.Stop() // 停止新的 Cron 调度
	if m.cancel != nil {
		m.cancel() // 取消 Worker 上下文，Worker 的 Pop 阻塞会立刻退出
	}
	// sync.Once 保护：Stop() 可能被优雅关闭流程多次调用（如 signal handler + main 退出），
	// 重复 close channel 会 panic。sync.Once 确保仅第一次调用执行 close。
	m.stopOnce.Do(func() {
		close(m.stopChan) // 通知 Interval 任务退出
	})
	if m.queue != nil {
		// queue.Close 失败仅 Warn：进程即将退出，残留资源由 OS 回收。
		if err := m.queue.Close(); err != nil {
			slog.Warn("task manager: queue.Close failed (process exiting, OS will reclaim)",
				"error", err)
		}
	}

	// 等待所有正在执行的任务完成 (包括 Interval 和正在跑的 Cron)
	m.wg.Wait()
	slog.Info("任务引擎所有任务已安全退出")
}

// getTaskMetadata 合并默认元数据与配置文件设置
func (m *Manager) getTaskMetadata(t Task) TaskMetadata {
	name := t.Name()

	// 1. 获取代码默认值
	var meta TaskMetadata
	if tm, ok := t.(TaskWithMetadata); ok {
		meta = tm.DefaultMetadata()
	} else {
		meta = TaskMetadata{
			Name:        name,
			DisplayName: t.DisplayName(),
			Type:        TypeOnce,
			Weight:      WeightNormal,
			Enabled:     false,
		}
	}

	// 2. 智能覆盖：仅当 config.toml 中有明确非 nil 定义时才覆盖
	if jobCfg, ok := m.cfg.Jobs[name]; ok {
		if jobCfg.Enabled != nil {
			meta.Enabled = *jobCfg.Enabled
		}
		if jobCfg.Type != nil {
			meta.Type = TaskType(*jobCfg.Type)
		}
		if jobCfg.Spec != nil {
			meta.Spec = *jobCfg.Spec
		}
		if jobCfg.Weight != nil {
			meta.Weight = *jobCfg.Weight
		}
	}

	return meta
}

// runIntervalTask 运行间隔任务
func (m *Manager) runIntervalTask(ctx context.Context, t Task, meta TaskMetadata, stopChan chan struct{}) {
	defer m.wg.Done()

	d, err := time.ParseDuration(meta.Spec)
	if err != nil {
		slog.Error("任务间隔参数无效", "name", meta.Name, "spec", meta.Spec, "error", err)
		return
	}

	ticker := time.NewTicker(d)
	defer ticker.Stop()

	slog.Info("间隔任务已启动", "name", meta.Name, "spec", meta.Spec)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-stopChan:
			return
		case <-ticker.C:
			m.execute(ctx, t)
		}
	}
}

// registerCronTask 注册定时任务
func (m *Manager) registerCronTask(ctx context.Context, t Task, meta TaskMetadata) {
	entryID, err := m.cron.AddFunc(meta.Spec, func() {
		// 生产级增强：定时任务进入 WaitGroup 保护，防止进程退出时任务被腰斩
		m.wg.Add(1)
		// GoSafe 包裹 recover + Sentry 上报，防止定时任务 panic 影响调度引擎。
		// defer m.wg.Done() 在 fn 内部，panic 时仍会触发（defer 在 recover 捕获前执行）。
		recovery.GoSafe("task:cron", func() {
			defer m.wg.Done()
			m.execute(ctx, t)
		})
	})
	if err != nil {
		slog.Error("任务 Cron 表达式无效", "name", meta.Name, "spec", meta.Spec, "error", err)
		return
	}

	m.mu.Lock()
	m.cronIDs[meta.Name] = entryID
	m.mu.Unlock()

	slog.Info("定时任务已注册", "name", meta.Name, "spec", meta.Spec)
}
