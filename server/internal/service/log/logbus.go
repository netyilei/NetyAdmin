package log

import (
	"context"
	"log"
	"sync"
	"time"

	logEntity "NetyAdmin/internal/domain/entity/log"
	"NetyAdmin/internal/pkg/configsync"
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
	go s.loop()

	return s
}

func (b *logBusService) loadConfig() {
	if val, ok := b.watcher.GetConfig("logbus_config", "global_max_entries"); ok {
		if v := parseInt(val); v > 0 {
			b.globalMaxEntries = v
		}
	}
	if b.globalMaxEntries == 0 {
		b.globalMaxEntries = 2000
	}

	if val, ok := b.watcher.GetConfig("logbus_config", "global_max_bytes_mb"); ok {
		if v := parseInt(val); v > 0 {
			b.globalMaxBytesMB = v
		}
	}
	if b.globalMaxBytesMB == 0 {
		b.globalMaxBytesMB = 10
	}

	if val, ok := b.watcher.GetConfig("logbus_config", "force_sync"); ok {
		b.forceSync = val == "true"
	}

	for lt, cfg := range b.configs {
		groupKey := logTypeConfigKey(lt)
		if val, ok := b.watcher.GetConfig("logbus_config", groupKey+"_batch_size"); ok {
			if v := parseInt(val); v > 0 {
				cfg.SizeThreshold = v
			}
		}
		if val, ok := b.watcher.GetConfig("logbus_config", groupKey+"_time_threshold"); ok {
			if v := parseInt(val); v > 0 {
				cfg.TimeThreshold = time.Duration(v) * time.Second
			}
		}
		if cfg.SizeThreshold == 0 {
			if val, ok := b.watcher.GetConfig("logbus_config", "default_batch_size"); ok {
				if v := parseInt(val); v > 0 {
					cfg.SizeThreshold = v
				}
			}
			if cfg.SizeThreshold == 0 {
				cfg.SizeThreshold = 200
			}
		}
		if cfg.TimeThreshold == 0 {
			if val, ok := b.watcher.GetConfig("logbus_config", "default_time_threshold"); ok {
				if v := parseInt(val); v > 0 {
					cfg.TimeThreshold = time.Duration(v) * time.Second
				}
			}
			if cfg.TimeThreshold == 0 {
				cfg.TimeThreshold = 5 * time.Second
			}
		}
		b.configs[lt] = cfg
	}
}

func (b *logBusService) Record(ctx context.Context, entry logEntity.LogEntry) error {
	if b.forceSync {
		return b.syncWrite(ctx, entry)
	}

	cfg, exists := b.configs[entry.GetLogType()]
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
	_ = b.tryAppend(entry)
	b.mu.Unlock()
	return nil
}

func (b *logBusService) tryAppend(entry logEntity.LogEntry) bool {
	lt := entry.GetLogType()
	buf := b.buffers[lt]
	b.buffers[lt] = append(buf, entry)
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
		cfg := b.configs[lt]
		writer := b.writers[lt]
		if writer == nil {
			continue
		}

		if cfg.SizeThreshold > 0 && len(entries) >= cfg.SizeThreshold {
			b.flushToWriter(writer, entries)
		} else {
			b.flushToWriter(writer, entries)
		}
	}
}

func (b *logBusService) flushToWriter(writer LogBatchWriter, entries []logEntity.LogEntry) {
	if len(entries) == 0 {
		return
	}
	if err := writer.WriteBatch(context.Background(), entries); err != nil {
		log.Printf("[LogBus] flush failed (%d entries): %v", len(entries), err)
	}
}

func (b *logBusService) Stop() {
	close(b.stopChan)
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

func parseInt(s string) int {
	v := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return v
}
