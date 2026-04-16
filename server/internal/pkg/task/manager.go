package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/cache"
)

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
		log.Println("[任务引擎] 已启用 Redis 分布式队列驱动")
	} else {
		m.queue = NewLocalQueue(1000)
		log.Println("[任务引擎] 已启用本地 Channel 队列驱动")
	}

	return m
}

// SetOnFinish 设置任务执行完成后的回调（可用于持久化日志）
func (m *Manager) SetOnFinish(fn func(name string, info ExecutionInfo)) {
	m.onFinish = fn
}

// Register 注册一个或多个任务。如果任务名重复，后者将覆盖前者。
func (m *Manager) Register(tasks ...Task) {
	for _, t := range tasks {
		name := t.Name()
		if _, exists := m.tasks[name]; exists {
			log.Printf("[任务引擎] 警告: 任务 [%s] 重复注册，将使用最新实例", name)
		}
		m.tasks[name] = t
	}
}

// Start 启动调度引擎
func (m *Manager) Start(ctx context.Context) {
	if m.cfg == nil || !m.cfg.Enabled {
		log.Println("[任务引擎] 未启用，跳过启动")
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

	for _, t := range m.tasks {
		meta := m.getTaskMetadata(t)
		if meta.Enabled {
			enabledTasks = append(enabledTasks, taskWithConfig{t, meta})
		}
	}

	// 2. 按权重降序排序（确保 Once 任务按优先级执行）
	sort.Slice(enabledTasks, func(i, j int) bool {
		return enabledTasks[i].metadata.Weight > enabledTasks[j].metadata.Weight
	})

	log.Printf("[任务引擎] 启动中，共激活 %d 个任务", len(enabledTasks))

	// 3. 分类处理
	for _, tc := range enabledTasks {
		switch tc.metadata.Type {
		case TypeOnce:
			// Once 任务同步顺序执行 (生产者级别：必须等待系统任务完成)
			log.Printf("[任务引擎] 执行同步启动任务: %s (权重: %d)", tc.metadata.Name, tc.metadata.Weight)
			if err := tc.task.Run(ctx); err != nil {
				log.Printf("[任务引擎] 启动任务 [%s] 执行失败: %v", tc.metadata.Name, err)
			}
		case TypeInterval:
			m.wg.Add(1)
			stopChan := make(chan struct{})
			m.mu.Lock()
			m.intervals[tc.metadata.Name] = stopChan
			m.mu.Unlock()
			go m.runIntervalTask(ctx, tc.task, tc.metadata, stopChan)
		case TypeCron:
			m.registerCronTask(ctx, tc.task, tc.metadata)
		}
	}

	m.cron.Start()

	// 4. 启动后台消费者 Worker
	m.startWorkers(workerCtx)
}

// Dispatch 投递子任务 (实现 Dispatcher 接口)
func (m *Manager) Dispatch(ctx context.Context, taskName string, payload interface{}, weight int) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload failed: %w", err)
	}

	msg := &Message{
		TaskName: taskName,
		Payload:  data,
	}

	if err := m.queue.Push(ctx, msg, weight); err != nil {
		return fmt.Errorf("push message to queue failed: %w", err)
	}

	return nil
}

func (m *Manager) startWorkers(ctx context.Context) {
	// 默认启动 5 个 Worker，未来可改为配置化
	workerCount := 5
	log.Printf("[任务引擎] 启动 %d 个后台 Worker 处理队列任务", workerCount)

	for i := 0; i < workerCount; i++ {
		m.wg.Add(1)
		go func(workerID int) {
			defer m.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-m.stopChan:
					return
				default:
					msg, err := m.queue.Pop(ctx)
					if err != nil {
						log.Printf("[任务引擎] Worker-%d Pop 消息失败: %v", workerID, err)
						time.Sleep(time.Second) // 发生错误稍后重试
						continue
					}
					if msg == nil {
						continue // 超时无消息
					}

					m.executePayload(ctx, msg)
				}
			}
		}(i)
	}
}

func (m *Manager) executePayload(ctx context.Context, msg *Message) {
	m.mu.RLock()
	t, exists := m.tasks[msg.TaskName]
	m.mu.RUnlock()

	if !exists {
		log.Printf("[任务引擎] 消费者执行失败: 任务 [%s] 未注册", msg.TaskName)
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
		log.Printf("[任务引擎] 任务 [%s] 载荷执行失败: %v", msg.TaskName, err)
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
		go m.execute(ctx, t)
	case TypeInterval:
		m.wg.Add(1)
		stopChan := make(chan struct{})
		m.mu.Lock()
		m.intervals[name] = stopChan
		m.mu.Unlock()
		go m.runIntervalTask(ctx, t, meta, stopChan)
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
		log.Printf("[任务引擎] 间隔任务 [%s] 已发出停止信号", name)
		return nil
	}

	// 2. 处理 Cron 任务
	if entryID, exists := m.cronIDs[name]; exists {
		m.cron.Remove(entryID)
		delete(m.cronIDs, name)
		log.Printf("[任务引擎] 定时任务 [%s] 已从调度器移除", name)
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
	_ = m.StopTask(name)

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

	log.Printf("[任务引擎] 任务 [%s] 已处于禁用状态，无需启动", name)
	return nil
}

// ManualRun 手动触发任务执行
func (m *Manager) ManualRun(ctx context.Context, name string) error {
	m.mu.RLock()
	t, exists := m.tasks[name]
	m.mu.RUnlock()

	if !exists {
		return log.Output(2, "[任务引擎] 手动触发失败: 任务不存在")
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.execute(ctx, t)
	}()

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
		// 设置锁的 TTL 为 1 小时 (兜底时间)
		// 避免任务执行时间超过 60s 导致锁失效
		err := m.redis.SetArgs(ctx, lockKey, "locked", redis.SetArgs{
			Mode: "NX",
			TTL:  1 * time.Hour,
		}).Err()
		if err == redis.Nil {
			// 未抢到锁，说明其他实例正在执行
			log.Printf("[任务引擎] 任务 [%s] 在其他实例中执行，本实例跳过", name)

			// 把状态改回未运行
			m.mu.Lock()
			state.IsRunning = false
			m.mu.Unlock()
			return
		} else if err != nil {
			log.Printf("[任务引擎] 任务 [%s] 尝试获取分布式锁失败: %v", name, err)
			m.mu.Lock()
			state.IsRunning = false
			m.mu.Unlock()
			return // 发生错误，安全起见不执行
		}

		// 执行完毕后释放锁
		defer func() {
			_ = m.redis.Del(ctx, lockKey)
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
		m.onFinish(name, info)
	}
}

// Stop 停止引擎 (优雅停机)
func (m *Manager) Stop() {
	if m.cfg == nil || !m.cfg.Enabled {
		return
	}
	log.Println("[任务引擎] 正在发出停止信号...")
	m.cron.Stop() // 停止新的 Cron 调度
	if m.cancel != nil {
		m.cancel() // 取消 Worker 上下文，Worker 的 Pop 阻塞会立刻退出
	}
	close(m.stopChan) // 通知 Interval 任务退出
	if m.queue != nil {
		_ = m.queue.Close()
	}

	// 等待所有正在执行的任务完成 (包括 Interval 和正在跑的 Cron)
	m.wg.Wait()
	log.Println("[任务引擎] 所有任务已安全退出")
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
		log.Printf("[任务引擎] 任务 [%s] 间隔参数无效 [%s]: %v", meta.Name, meta.Spec, err)
		return
	}

	ticker := time.NewTicker(d)
	defer ticker.Stop()

	log.Printf("[任务引擎] 间隔任务 [%s] 已启动，周期: %s", meta.Name, meta.Spec)

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
		defer m.wg.Done()

		m.execute(ctx, t)
	})
	if err != nil {
		log.Printf("[任务引擎] 任务 [%s] Cron 表达式无效 [%s]: %v", meta.Name, meta.Spec, err)
		return
	}

	m.mu.Lock()
	m.cronIDs[meta.Name] = entryID
	m.mu.Unlock()

	log.Printf("[任务引擎] 定时任务 [%s] 已注册，表达式: %s", meta.Name, meta.Spec)
}
