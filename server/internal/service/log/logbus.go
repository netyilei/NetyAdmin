package log

import (
	"context"
	"sync"
	"time"

	logEntity "NetyAdmin/internal/domain/entity/log"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/recovery"
	"NetyAdmin/internal/pkg/requestid"
	"NetyAdmin/internal/pkg/slogutil"
	"NetyAdmin/internal/pkg/utils"
)

type LogBatchWriter interface {
	WriteBatch(ctx context.Context, entries []logEntity.LogEntry) error
}

type LogBusService interface {
	Record(ctx context.Context, entry logEntity.LogEntry) error
	RecordSync(ctx context.Context, entry logEntity.LogEntry) error
	Stop()
}

type BucketConfig struct {
	Priority      logEntity.LogPriority
	SizeThreshold int
	TimeThreshold time.Duration
}

type logBusService struct {
	writers  map[logEntity.LogType]LogBatchWriter
	configs  map[logEntity.LogType]BucketConfig
	watcher  configsync.ConfigWatcher
	buffers  map[logEntity.LogType][]logEntity.LogEntry
	mu       sync.Mutex
	stopChan chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once

	globalMaxEntries int
	globalMaxBytesMB int
	forceSync        bool
	totalEntries     int
}

func NewLogBusService(
	writers map[logEntity.LogType]LogBatchWriter,
	configs map[logEntity.LogType]BucketConfig,
	watcher configsync.ConfigWatcher,
) LogBusService {
	s := &logBusService{
		writers:  writers,
		configs:  configs,
		watcher:  watcher,
		buffers:  make(map[logEntity.LogType][]logEntity.LogEntry),
		stopChan: make(chan struct{}),
	}

	for lt := range writers {
		s.buffers[lt] = make([]logEntity.LogEntry, 0)
	}

	s.loadConfig()

	s.wg.Add(1)
	// 异步启动日志刷盘循环（GoSafe 包裹 recover + Sentry 上报，防止 panic 导致日志总线静默退出）
	// s.loop 内部 defer b.wg.Done() 在 panic 时仍会触发（defer 在 recover 捕获前执行）
	recovery.GoSafe("logbus:loop", s.loop)

	return s
}

func (b *logBusService) loadConfig() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 全局配置：maxEntries / maxBytesMB（默认 2000 / 10MB）
	if v := utils.GetIntWithDefault(b.watcher, "logbus_config", "global_max_entries", 2000); v > 0 {
		b.globalMaxEntries = v
	}
	if v := utils.GetIntWithDefault(b.watcher, "logbus_config", "global_max_bytes_mb", 10); v > 0 {
		b.globalMaxBytesMB = v
	}

	if val, ok := b.watcher.GetConfig("logbus_config", "force_sync"); ok {
		b.forceSync = val == "true"
	}

	for lt, cfg := range b.configs {
		groupKey := logTypeConfigKey(lt)

		// 各日志类型独立配置（优先级高于默认）
		if v := utils.GetIntWithDefault(b.watcher, "logbus_config", groupKey+"_batch_size", 0); v > 0 {
			cfg.SizeThreshold = v
		}
		if v := utils.GetIntWithDefault(b.watcher, "logbus_config", groupKey+"_time_threshold", 0); v > 0 {
			cfg.TimeThreshold = time.Duration(v) * time.Second
		}
		// 兜底默认值（未配置独立项时用全局默认）
		if cfg.SizeThreshold == 0 {
			cfg.SizeThreshold = utils.GetIntWithDefault(b.watcher, "logbus_config", "default_batch_size", 200)
		}
		if cfg.TimeThreshold == 0 {
			defaultSec := utils.GetIntWithDefault(b.watcher, "logbus_config", "default_time_threshold", 5)
			cfg.TimeThreshold = time.Duration(defaultSec) * time.Second
		}
		b.configs[lt] = cfg
	}
}

func (b *logBusService) Record(ctx context.Context, entry logEntity.LogEntry) error {
	b.mu.Lock()
	forceSync := b.forceSync
	b.mu.Unlock()

	if forceSync {
		return b.syncWrite(ctx, entry)
	}

	b.mu.Lock()
	cfg, exists := b.configs[entry.GetLogType()]
	b.mu.Unlock()
	if !exists {
		return b.syncWrite(ctx, entry)
	}

	b.mu.Lock()
	if b.totalEntries >= b.globalMaxEntries {
		b.evictOldest(logEntity.PriorityP2)
		if b.totalEntries >= b.globalMaxEntries {
			b.evictOldest(logEntity.PriorityP1)
		}
		if b.totalEntries >= b.globalMaxEntries && cfg.Priority == logEntity.PriorityP0 {
			b.mu.Unlock()
			return b.syncWrite(ctx, entry)
		}
	}
	b.mu.Unlock()

	switch cfg.Priority {
	case logEntity.PriorityP0:
		return b.submitP0(ctx, entry)
	case logEntity.PriorityP1:
		return b.submitP1(ctx, entry)
	default:
		return b.submitP2(entry)
	}
}

func (b *logBusService) RecordSync(ctx context.Context, entry logEntity.LogEntry) error {
	return b.syncWrite(ctx, entry)
}

func (b *logBusService) submitP0(ctx context.Context, entry logEntity.LogEntry) error {
	b.mu.Lock()
	added := b.tryAppend(entry)
	b.mu.Unlock()
	if added {
		return nil
	}
	return b.syncWrite(ctx, entry)
}

func (b *logBusService) submitP1(ctx context.Context, entry logEntity.LogEntry) error {
	b.mu.Lock()
	added := b.tryAppend(entry)
	b.mu.Unlock()
	if added {
		return nil
	}

	time.Sleep(50 * time.Millisecond)

	b.mu.Lock()
	added = b.tryAppend(entry)
	b.mu.Unlock()
	if added {
		return nil
	}

	return b.syncWrite(ctx, entry)
}

func (b *logBusService) submitP2(entry logEntity.LogEntry) error {
	b.mu.Lock()
	added := b.tryAppend(entry)
	b.mu.Unlock()
	if !added {
		// Task 8.3: 用 slogutil.LoggerFromContext 携带 request_id，
		// 便于通过 request_id 关联到触发该日志的原始请求。
		ctx := context.Background()
		if rid := entry.GetRequestID(); rid != "" {
			ctx = requestid.WithRequestID(ctx, rid)
		}
		slogutil.LoggerFromContext(ctx).Warn("LogBus P2 日志丢弃", "type", entry.GetLogType())
	}
	return nil
}

func (b *logBusService) tryAppend(entry logEntity.LogEntry) bool {
	lt := entry.GetLogType()
	cfg, exists := b.configs[lt]
	if !exists {
		return false
	}
	// 缓冲区达到单批次阈值时拒绝追加，触发调用方的同步写入降级
	if cfg.SizeThreshold > 0 && len(b.buffers[lt]) >= cfg.SizeThreshold {
		return false
	}
	b.buffers[lt] = append(b.buffers[lt], entry)
	b.totalEntries++
	return true
}

func (b *logBusService) evictOldest(minPriority logEntity.LogPriority) {
	for lt, cfg := range b.configs {
		if cfg.Priority > minPriority {
			continue
		}
		buf := b.buffers[lt]
		if len(buf) > 0 {
			b.buffers[lt] = buf[1:]
			b.totalEntries--
			return
		}
	}
}

func (b *logBusService) syncWrite(ctx context.Context, entry logEntity.LogEntry) error {
	writer, exists := b.writers[entry.GetLogType()]
	if !exists {
		return nil
	}
	return writer.WriteBatch(ctx, []logEntity.LogEntry{entry})
}

func (b *logBusService) loop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.minTimeThreshold())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flushAll()
			b.loadConfig()
			ticker.Reset(b.minTimeThreshold())
		case <-b.stopChan:
			b.flushAll()
			return
		}
	}
}

func (b *logBusService) minTimeThreshold() time.Duration {
	min := time.Hour
	for _, cfg := range b.configs {
		if cfg.TimeThreshold < min {
			min = cfg.TimeThreshold
		}
	}
	return min
}

func (b *logBusService) flushAll() {
	b.mu.Lock()
	snapshots := make(map[logEntity.LogType][]logEntity.LogEntry)
	for lt, buf := range b.buffers {
		if len(buf) > 0 {
			snapshots[lt] = buf
			b.buffers[lt] = make([]logEntity.LogEntry, 0)
		}
	}
	b.totalEntries = 0
	b.mu.Unlock()

	for lt, entries := range snapshots {
		writer := b.writers[lt]
		if writer == nil {
			continue
		}
		b.flushToWriter(writer, entries)
	}
}

func (b *logBusService) flushToWriter(writer LogBatchWriter, entries []logEntity.LogEntry) {
	if len(entries) == 0 {
		return
	}
	// Task 8.3: 用 slogutil.LoggerFromContext 替代裸 slog，让日志自动携带 request_id。
	// 异步刷盘场景下原始请求 ctx 早已不可用，从首条 entry 的 RequestID 恢复到子 ctx，
	// 便于通过 request_id 关联到触发该批日志的原始请求。
	// 若所有 entry 均无 RequestID，LoggerFromContext 返回 slog.Default()（保持原行为）。
	ctx := context.Background()
	if rid := entries[0].GetRequestID(); rid != "" {
		ctx = requestid.WithRequestID(ctx, rid)
	}
	logger := slogutil.LoggerFromContext(ctx)
	if err := writer.WriteBatch(ctx, entries); err != nil {
		logger.Error("LogBus flush failed", "count", len(entries), "error", err)
	}
}

func (b *logBusService) Stop() {
	b.stopOnce.Do(func() { close(b.stopChan) })
	b.wg.Wait()
}

func logTypeConfigKey(lt logEntity.LogType) string {
	switch lt {
	case logEntity.LogTypeOperation:
		return "operation"
	case logEntity.LogTypeError:
		return "error"
	case logEntity.LogTypeOpen:
		return "open"
	case logEntity.LogTypeTask:
		return "task"
	default:
		return string(lt)
	}
}
