package log

import (
	"context"
	"crypto/md5"
	"fmt"
	"runtime"
	"time"

	logEntity "NetyAdmin/internal/domain/entity/log"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	logRepo "NetyAdmin/internal/repository/log"
)

type ErrorService interface {
	Log(ctx context.Context, log *logEntity.Error) error
	LogPanic(ctx context.Context, err interface{}, requestID, path, method, ip, userAgent string, adminID uint)
	LogError(ctx context.Context, err error, requestID, path, method, ip, userAgent string, adminID uint)
	List(ctx context.Context, level string, resolved *bool, page, pageSize int) ([]logEntity.Error, int64, error)
	Resolve(ctx context.Context, id, resolvedBy uint) error
	Delete(ctx context.Context, id uint) error
	DeleteBatch(ctx context.Context, ids []uint) error
}

type errorService struct {
	logRepo       *logRepo.ErrorRepository
	configWatcher configsync.ConfigWatcher
	cache         cache.LazyCacheManager
}

func NewErrorService(logRepo *logRepo.ErrorRepository, configWatcher configsync.ConfigWatcher, cacheMgr cache.LazyCacheManager) ErrorService {
	return &errorService{
		logRepo:       logRepo,
		configWatcher: configWatcher,
		cache:         cacheMgr,
	}
}

func (s *errorService) Log(ctx context.Context, logRecord *logEntity.Error) error {
	logRecord.Hash = s.generateHash(logRecord)
	logRecord.LastOccurredAt = time.Now()

	useCache := s.configWatcher.IsCacheEnabled("err_log_cache")

	if useCache && s.cache != nil {
		cacheKey := cache.KeyErrorLogSuppress(logRecord.Hash)

		set, err := s.cache.SetNX(ctx, cacheKey, "1", 60*time.Second)
		if err == nil && !set {
			return nil
		}
	}

	return s.logRepo.UpsertByHash(ctx, logRecord)
}

func (s *errorService) LogPanic(ctx context.Context, err interface{}, requestID, path, method, ip, userAgent string, adminID uint) {
	stack := s.getStack(3)

	logRecord := &logEntity.Error{
		Level:     logEntity.LogLevelPanic,
		Message:   fmt.Sprintf("%v", err),
		Stack:     stack,
		RequestID: requestID,
		Path:      path,
		Method:    method,
		AdminID:   adminID,
		IP:        ip,
		UserAgent: userAgent,
	}

	_ = s.Log(ctx, logRecord)
}

func (s *errorService) LogError(ctx context.Context, err error, requestID, path, method, ip, userAgent string, adminID uint) {
	stack := s.getStack(3)

	logRecord := &logEntity.Error{
		Level:     logEntity.LogLevelError,
		Message:   err.Error(),
		Stack:     stack,
		RequestID: requestID,
		Path:      path,
		Method:    method,
		AdminID:   adminID,
		IP:        ip,
		UserAgent: userAgent,
	}

	_ = s.Log(ctx, logRecord)
}

func (s *errorService) generateHash(l *logEntity.Error) string {
	// 提取核心堆栈（前 3 行），避免因为行号偏移导致的指纹失效（如果代码变动剧烈）
	// 但通常直接取全部 Stack 也是可以的，这里为了严谨做下简单切分
	stackKey := l.Stack
	if len(l.Stack) > 500 {
		stackKey = l.Stack[:500]
	}

	// 特征因子：级别 + 消息 + 路径 + 方法 + 核心堆栈
	raw := fmt.Sprintf("%s|%s|%s|%s|%s", l.Level, l.Message, l.Path, l.Method, stackKey)
	return fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

func (s *errorService) getStack(skip int) string {
	var stack string
	for i := skip; ; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		stack += fmt.Sprintf("%s:%d\n", file, line)
	}
	return stack
}

func (s *errorService) List(ctx context.Context, level string, resolved *bool, page, pageSize int) ([]logEntity.Error, int64, error) {
	return s.logRepo.List(ctx, level, resolved, page, pageSize)
}

func (s *errorService) Resolve(ctx context.Context, id, resolvedBy uint) error {
	return s.logRepo.Resolve(ctx, id, resolvedBy)
}

func (s *errorService) Delete(ctx context.Context, id uint) error {
	return s.logRepo.Delete(ctx, id)
}

func (s *errorService) DeleteBatch(ctx context.Context, ids []uint) error {
	return s.logRepo.DeleteBatch(ctx, ids)
}
