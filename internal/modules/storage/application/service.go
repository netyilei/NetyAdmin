// service.go：存储服务——配置管理/凭证签发/complete 回调/
// 过期清理 Job。驱动=每配置一 S3 实例（惰性构造+缓存）；凭证 HMAC=SHA256
// (record.ID + objectKey + 盐)——complete 校验防伪造。盐 env 注入（bootstrap）。
package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/storage/domain"
)

// DefaultCredentialsTTL 等默认基线（config 可覆盖——G1-08 具名）。
const (
	DefaultCredentialsTTL  = 15 * time.Minute
	DefaultCleanupInterval = 10 * time.Minute
	DefaultCompleteGrace   = 5 * time.Minute
)

// Config 是存储模块参数（config.storage）。
type Config struct {
	CredentialsTTL  time.Duration // 凭证有效期（默认 15min）
	CleanupInterval time.Duration // 清理 Job 间隔（默认 10min）
	CompleteGrace   time.Duration // complete 宽限（默认 5min）
	HMACSalt        string        // complete 防伪盐（env Secret——bootstrap 注入）
}

// RepoPair 聚合 repo（AttachDB 注入）。
type RepoPair struct {
	Configs StorageConfigRepo
	Records UploadRecordRepo
}

// StorageConfigRepo 是配置持久化契约。
type StorageConfigRepo interface {
	Create(ctx context.Context, c *domain.StorageConfig) error
	Update(ctx context.Context, c *domain.StorageConfig) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*domain.StorageConfig, error)
	GetDefault(ctx context.Context) (*domain.StorageConfig, error)
	SetDefault(ctx context.Context, id string) error
	List(ctx context.Context, offset, limit int) ([]*domain.StorageConfig, int, error)
}

// UploadRecordRepo 是上传记录持久化契约。
type UploadRecordRepo interface {
	Create(ctx context.Context, r *domain.UploadRecord) error
	Get(ctx context.Context, id string) (*domain.UploadRecord, error)
	MarkCompleted(ctx context.Context, id, fileURL string, completedAt time.Time) (bool, error) // CAS：pending→completed
	ExpireCandidates(ctx context.Context, before time.Time) ([]*domain.UploadRecord, error)     // 过期 pending 候选（cleanup 先删对象后置态——B3）
	ExpireOne(ctx context.Context, id string) error                                             // 单条 CAS pending→expired
	List(ctx context.Context, offset, limit int) ([]*domain.UploadRecord, int, error)
}

// 领域哨兵（repo 层转译——adapters mapStorageError 消费）。
var (
	ErrConfigNotFound    = domain.ErrConfigNotFound
	ErrRecordNotFound    = domain.ErrRecordNotFound
	ErrSecretMismatch    = domain.ErrSecretMismatch
	ErrAlreadyCompleted  = domain.ErrAlreadyCompleted
	ErrNoDefaultConfig   = errors.New("storage: no default config")
	ErrConfigDisabled    = errors.New("storage: config disabled") // 默认配置被禁用/删除——区别于「无默认」（C3）
	ErrExpiredWindow     = errors.New("storage: complete window expired")
	ErrHMACNotConfigured = errors.New("storage: hmac salt not configured") // 盐空=complete 防伪失效——fail-closed（联测审计 A1）
	ErrConfigConflict    = domain.ErrConfigConflict
	ErrProviderInvalid   = domain.ErrProviderInvalid
)

// Service 是存储服务（配置/凭证/complete/清理）。
type Service struct {
	cfg      Config
	hmacSalt string
	repo     *RepoPair
	logger   *slog.Logger
	now      func() time.Time
	ulid     func() string
	mu       sync.Mutex
	drivers  map[string]domain.StorageDriver // configID → 驱动实例（惰性构造+缓存）
}

// New 创建（hmacSalt=complete 防伪盐 env 注入——空=complete 全拒 fail-closed）。
func New(cfg Config, hmacSalt string, logger *slog.Logger) *Service {
	if cfg.CredentialsTTL <= 0 {
		cfg.CredentialsTTL = DefaultCredentialsTTL
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultCleanupInterval
	}
	if cfg.CompleteGrace <= 0 {
		cfg.CompleteGrace = DefaultCompleteGrace
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		cfg: cfg, hmacSalt: hmacSalt, logger: logger,
		drivers: map[string]domain.StorageDriver{},
		now:     time.Now,
		ulid:    func() string { return primitives.NewULID(primitives.SystemClock{}) },
	}
}

// Attach 注入 repo（DB 连接后——模块契约）。
func (s *Service) Attach(repo *RepoPair) { s.repo = repo }

func (s *Service) repoOK() error {
	if s.repo == nil {
		return errors.New("storage: repo not attached")
	}
	return nil
}

// —— 配置——

// CreateConfig 创建配置（provider/PathPrefix 校验；is_default=true 先清后设）。
func (s *Service) CreateConfig(ctx context.Context, c *domain.StorageConfig) error {
	if err := s.repoOK(); err != nil {
		return err
	}
	if !c.Provider.Valid() {
		return domain.ErrProviderInvalid
	}
	if !c.BucketLookup.Valid() {
		return domain.ErrBucketLookupInvalid
	}
	if err := domain.ValidatePathPrefix(c.PathPrefix); err != nil {
		return err
	}
	c.ID = s.ulid()
	now := s.now()
	c.CreatedAt, c.UpdatedAt = now, now
	if err := s.repo.Configs.Create(ctx, c); err != nil {
		return err
	}
	// 先建行后设默认（顺序修正：SetDefault 先于 Create=行不存在 404）
	if c.IsDefault {
		return s.repo.Configs.SetDefault(ctx, c.ID)
	}
	return nil
}

// UpdateConfig 更新配置（provider/PathPrefix 校验；驱动缓存失效——下次使用重建）。
func (s *Service) UpdateConfig(ctx context.Context, c *domain.StorageConfig) error {
	if err := s.repoOK(); err != nil {
		return err
	}
	if !c.Provider.Valid() {
		return domain.ErrProviderInvalid
	}
	if !c.BucketLookup.Valid() {
		return domain.ErrBucketLookupInvalid
	}
	if err := domain.ValidatePathPrefix(c.PathPrefix); err != nil {
		return err
	}
	c.UpdatedAt = s.now()
	if err := s.repo.Configs.Update(ctx, c); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.drivers, c.ID)
	s.mu.Unlock()
	return nil
}

// DeleteConfig 删除配置（驱动缓存失效）。
func (s *Service) DeleteConfig(ctx context.Context, id string) error {
	if err := s.repoOK(); err != nil {
		return err
	}
	if err := s.repo.Configs.Delete(ctx, id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.drivers, id)
	s.mu.Unlock()
	return nil
}

// SetDefault 设默认。
func (s *Service) SetDefault(ctx context.Context, id string) error {
	if err := s.repoOK(); err != nil {
		return err
	}
	return s.repo.Configs.SetDefault(ctx, id)
}

// GetConfig 取配置。
func (s *Service) GetConfig(ctx context.Context, id string) (*domain.StorageConfig, error) {
	if err := s.repoOK(); err != nil {
		return nil, err
	}
	return s.repo.Configs.Get(ctx, id)
}

// ListConfigs 配置列表。
func (s *Service) ListConfigs(ctx context.Context, offset, limit int) ([]*domain.StorageConfig, int, error) {
	if err := s.repoOK(); err != nil {
		return nil, 0, err
	}
	return s.repo.Configs.List(ctx, offset, limit)
}

// —— 凭证/complete——

// CredentialsFor 为目标配置签发直传凭证（objectKey=prefix/ULID——fileName
// 不参与 key 防路径穿越；secret=HMAC(record.ID|objectKey)——complete 校验）。
func (s *Service) CredentialsFor(ctx context.Context, configID, fileName, mimeType, businessType, businessID, uploaderID, uploaderIP string) (*domain.Credentials, *domain.UploadRecord, error) {
	if err := s.repoOK(); err != nil {
		return nil, nil, err
	}
	cfg, err := s.resolveConfig(ctx, configID)
	if err != nil {
		return nil, nil, err
	}
	if s.hmacSalt == "" {
		// 盐空=HMAC 防伪退化（伪造 secret 可过 complete）——签发面直接拒绝
		// fail-closed（联测审计 A1：env 未接线时盐恒空，防伪为假绿）。
		return nil, nil, ErrHMACNotConfigured
	}
	if cfg.AllowedTypes != "" && !typeAllowed(cfg.AllowedTypes, mimeType) {
		return nil, nil, fmt.Errorf("%w: mime %q not allowed", domain.ErrProviderInvalid, mimeType)
	}
	driver, err := s.driverForConfigID(ctx, cfg.ID)
	if err != nil {
		return nil, nil, err
	}
	id := s.ulid()
	objectKey := s.objectKeyFor(cfg, id)
	presigned, err := driver.PresignPut(ctx, objectKey, mimeType, s.cfg.CredentialsTTL)
	if err != nil {
		return nil, nil, err
	}
	secret := s.hmacFor(id, objectKey)
	rec := &domain.UploadRecord{
		ID: id, ConfigID: cfg.ID, ObjectKey: objectKey, FileName: fileName,
		MimeType: mimeType, BusinessType: businessType, BusinessID: businessID,
		Secret: secret, Status: domain.RecordPending, UploaderID: uploaderID,
		UploaderIP: uploaderIP, CreatedAt: s.now(),
	}
	if err := s.repo.Records.Create(ctx, rec); err != nil {
		return nil, nil, err
	}
	return &domain.Credentials{
		RecordID: rec.ID, UploadURL: presigned.URL, ObjectKey: objectKey,
		Secret: secret, Headers: presigned.Headers, ExpiresAt: presigned.ExpiresAt,
	}, rec, nil
}

// Complete 完成回调（HMAC 校验 + CAS pending→completed——防重放/防双写；
// 宽限窗口=TTL+Grace，超窗拒绝）。
func (s *Service) Complete(ctx context.Context, recordID, secret string) (*domain.UploadRecord, error) {
	if err := s.repoOK(); err != nil {
		return nil, err
	}
	rec, err := s.repo.Records.Get(ctx, recordID)
	if err != nil {
		return nil, err
	}
	if rec.Status != domain.RecordPending {
		return nil, ErrAlreadyCompleted
	}
	if !s.verifyHMAC(rec.ID, rec.ObjectKey, secret) {
		return nil, ErrSecretMismatch
	}
	if s.now().Sub(rec.CreatedAt) > s.cfg.CredentialsTTL+s.cfg.CompleteGrace {
		return nil, ErrExpiredWindow
	}
	driver, err := s.driverForConfigID(ctx, rec.ConfigID)
	if err != nil {
		return nil, err
	}
	fileURL := driver.PublicURL(rec.ObjectKey)
	now := s.now()
	ok, err := s.repo.Records.MarkCompleted(ctx, rec.ID, fileURL, now)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrAlreadyCompleted
	}
	rec.Status, rec.FileURL, rec.CompletedAt = domain.RecordCompleted, fileURL, &now
	return rec, nil
}

// resolveConfig 取目标配置（configID 空=默认；禁用/删除细分 ErrConfigDisabled
// ——C3 语义修正：不再与「无默认」混报）。
func (s *Service) resolveConfig(ctx context.Context, configID string) (*domain.StorageConfig, error) {
	if configID != "" {
		cfg, err := s.repo.Configs.Get(ctx, configID)
		if err != nil {
			return nil, err
		}
		if cfg.Status != 1 {
			return nil, ErrConfigDisabled
		}
		return cfg, nil
	}
	cfg, err := s.repo.Configs.GetDefault(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.Status != 1 {
		return nil, ErrConfigDisabled
	}
	return cfg, nil
}

// driverForConfigID 惰性构造并缓存驱动（mu 保护 map）。
func (s *Service) driverForConfigID(ctx context.Context, configID string) (domain.StorageDriver, error) {
	s.mu.Lock()
	if d, ok := s.drivers[configID]; ok {
		s.mu.Unlock()
		return d, nil
	}
	s.mu.Unlock()
	cfg, err := s.repo.Configs.Get(ctx, configID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.drivers[configID]; ok {
		return d, nil
	}
	d, err := newS3Driver(*cfg)
	if err != nil {
		return nil, err
	}
	s.drivers[configID] = d
	return d, nil
}

// hmacFor/verifyHMAC：凭证 HMAC（SHA256——盐 env 注入；防伪造 complete）。
func (s *Service) hmacFor(recordID, objectKey string) string {
	mac := hmac.New(sha256.New, []byte(s.hmacSalt))
	fmt.Fprintf(mac, "%s|%s", recordID, objectKey)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// verifyHMAC 盐空恒 false（无盐=无法验证=拒绝——complete 面 fail-closed）。
func (s *Service) verifyHMAC(recordID, objectKey, secret string) bool {
	if s.hmacSalt == "" {
		return false
	}
	want := s.hmacFor(recordID, objectKey)
	return hmac.Equal([]byte(want), []byte(secret))
}

// objectKey 拼接（路径前缀 + ULID——fileName 不参与 key 防路径穿越）。
func (s *Service) objectKeyFor(cfg *domain.StorageConfig, id string) string {
	if cfg.PathPrefix != "" {
		return strings.Trim(cfg.PathPrefix, "/") + "/" + id
	}
	return id
}

// CleanupExpired 过期 pending 清理（cleanup Job/手动端点消费）：
// ①候选窗口=TTL+Grace（与 complete 窗口一致——B2：原 TTL 提前清扫会击穿
// 合法 complete 窗口误报 409）；②先删对象再置 expired（B3：原实现只改状态，
// S3 孤儿对象永不删除）；驱动不可用（配置已删/构造失败）时跳过对象删除、
// 仅置态推进（对象残留由桶生命周期兜底——注释声明边界）；③ctx 取消/超时
// 即中止并返回已处理数+错误（scheduler 60s 超时/停机面——禁吞取消假成功）。
func (s *Service) CleanupExpired(ctx context.Context) (int64, error) {
	if err := s.repoOK(); err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	before := s.now().Add(-(s.cfg.CredentialsTTL + s.cfg.CompleteGrace))
	cands, err := s.repo.Records.ExpireCandidates(ctx, before)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, rec := range cands {
		if cerr := ctx.Err(); cerr != nil {
			return n, fmt.Errorf("storage: cleanup interrupted after %d expired: %w", n, cerr)
		}
		if d, derr := s.driverForConfigID(ctx, rec.ConfigID); derr == nil {
			if delErr := d.Delete(ctx, rec.ObjectKey); delErr != nil {
				s.logger.Warn("storage: cleanup delete object failed, keep pending", "record", rec.ID,
					"object", rec.ObjectKey, "err", delErr)
				continue // 删除失败保 pending——下轮再扫（防孤儿对象永久残留）
			}
		} else {
			s.logger.Warn("storage: cleanup driver unavailable, expire without delete",
				"record", rec.ID, "config", rec.ConfigID, "err", derr)
		}
		if expErr := s.repo.Records.ExpireOne(ctx, rec.ID); expErr != nil {
			s.logger.Warn("storage: cleanup expire failed", "record", rec.ID, "err", expErr)
			continue
		}
		n++
	}
	return n, nil
}

// ListRecords 上传记录列表（管理端）。
func (s *Service) ListRecords(ctx context.Context, offset, limit int) ([]*domain.UploadRecord, int, error) {
	if err := s.repoOK(); err != nil {
		return nil, 0, err
	}
	return s.repo.Records.List(ctx, offset, limit)
}

// typeAllowed MIME 白名单判定（image/* 通配）。
func typeAllowed(allowed, mime string) bool {
	for _, a := range strings.Split(allowed, ",") {
		a = strings.TrimSpace(a)
		if a == mime || (strings.HasSuffix(a, "/*") && strings.HasPrefix(mime, strings.TrimSuffix(a, "*"))) {
			return true
		}
	}
	return false
}
