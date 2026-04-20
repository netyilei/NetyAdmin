package open_platform

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"NetyAdmin/internal/domain/entity/open_platform"
	"NetyAdmin/internal/pkg/configsync"
	openRepo "NetyAdmin/internal/repository/open_platform"
)

type OpenLogService interface {
	Record(ctx context.Context, log *open_platform.OpenPlatformLog) error
	ListLogs(ctx context.Context, query *openRepo.OpenLogRepoQuery) ([]*open_platform.OpenPlatformLog, int64, error)
	GetLog(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error)
	DeleteBatch(ctx context.Context, ids []uint64) error
	ClearOldLogs(ctx context.Context, days int) error
	Stop() // 优雅关闭
}

type openLogService struct {
	repo    openRepo.OpenLogRepository
	watcher configsync.ConfigWatcher

	logChan  chan *open_platform.OpenPlatformLog
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewOpenLogService(repo openRepo.OpenLogRepository, watcher configsync.ConfigWatcher) OpenLogService {
	s := &openLogService{
		repo:     repo,
		watcher:  watcher,
		logChan:  make(chan *open_platform.OpenPlatformLog, 1000),
		stopChan: make(chan struct{}),
	}

	s.wg.Add(1)
	go s.processLogs()

	return s
}

func (s *openLogService) Record(ctx context.Context, log *open_platform.OpenPlatformLog) error {
	select {
	case s.logChan <- log:
		return nil
	default:
		// 缓冲区满，为了不阻塞主流程，直接异步写入或丢弃（这里选择异步写入，虽然会有性能抖动，但保证数据不丢）
		go func() {
			_ = s.repo.Create(context.Background(), log)
		}()
		return nil
	}
}

func (s *openLogService) processLogs() {
	defer s.wg.Done()

	var buffer []*open_platform.OpenPlatformLog
	
	// 默认配置
	batchSize := 100
	interval := 5 * time.Second

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	flush := func() {
		if len(buffer) == 0 {
			return
		}
		
		// 批量插入
		// 注意：OpenLogRepository 暂无批量插入接口，我们需要去 repository 增加一个
		// 或者循环调用 Create，但批量插入效率更高。
		// 这里先写逻辑，稍后去 repository 增加 BatchCreate。
		if err := s.repo.BatchCreate(context.Background(), buffer); err != nil {
			log.Printf("[OpenLogService] Batch insert logs failed: %v", err)
		}
		buffer = make([]*open_platform.OpenPlatformLog, 0, batchSize)
	}

	for {
		// 动态获取配置
		if val, ok := s.watcher.GetConfig("open_platform_config", "log_buffer_size"); ok {
			if v, err := strconv.Atoi(val); err == nil && v > 0 {
				batchSize = v
			}
		}
		if val, ok := s.watcher.GetConfig("open_platform_config", "log_buffer_interval"); ok {
			if v, err := strconv.Atoi(val); err == nil && v > 0 {
				newInterval := time.Duration(v) * time.Second
				if newInterval != interval {
					interval = newInterval
					ticker.Reset(interval)
				}
			}
		}

		select {
		case logItem := <-s.logChan:
			buffer = append(buffer, logItem)
			if len(buffer) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-s.stopChan:
			flush()
			return
		}
	}
}

func (s *openLogService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *openLogService) ListLogs(ctx context.Context, query *openRepo.OpenLogRepoQuery) ([]*open_platform.OpenPlatformLog, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *openLogService) GetLog(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *openLogService) DeleteBatch(ctx context.Context, ids []uint64) error {
	return s.repo.DeleteBatch(ctx, ids)
}

func (s *openLogService) ClearOldLogs(ctx context.Context, days int) error {
	return s.repo.Clear(ctx, days)
}
