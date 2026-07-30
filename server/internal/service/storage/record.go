package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	storageEntity "NetyAdmin/internal/domain/entity/storage"
	storageVO "NetyAdmin/internal/domain/vo/storage"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/storage"
	"NetyAdmin/internal/pkg/utils"
	storageRepo "NetyAdmin/internal/repository/storage"
	openSvcPkg "NetyAdmin/internal/service/open_platform"
)

// credentialTTL 凭证（pending 记录）有效期：超过此时间未收到上传成功通知则视为过期。
const credentialTTL = 30 * time.Minute

// CredentialsRequest 是 service 层的上传凭证入参契约，与 admin/client DTO 解耦。
// admin/client handler 在调用 service 前需自行将各自 DTO 转换为本结构体。
type CredentialsRequest struct {
	ConfigID     uint
	FileName     string
	ContentType  string
	FileSize     int64
	BusinessType string
	BusinessID   string
	SourceInfo   map[string]interface{}
}

// CredentialsResult 是 service 层的上传凭证返回契约，与 admin/client DTO 解耦。
// 各端 handler 需自行将本结构体映射为对应响应 DTO。
type CredentialsResult struct {
	URL         string
	Method      string
	Headers     map[string]string
	ExpiresAt   time.Time
	ObjectKey   string
	Domain      string
	FinalURL    string
	ConfigID    uint
	Region      string
	Bucket      string
	Endpoint    string
	PathPrefix  string
	MaxFileSize int64
	RecordID    uint
	Secret      string
}

// RecordListRequest 是 service 层的上传记录列表查询入参契约，与 admin/client DTO 解耦。
// admin/client handler 在调用 service 前需自行将各自 DTO 转换为本结构体。
// 字段类型参照 admin DTO storageDto.RecordQuery（StartTime/EndTime 在 DTO 侧为字符串日期）。
type RecordListRequest struct {
	FileName        string
	Source          string
	SourceID        string
	BusinessType    string
	BusinessID      string
	MimeType        string
	StorageConfigID uint
	AppID           string
	StartTime       string
	EndTime         string
	Current         int
	Size            int
}

type RecordService interface {
	List(ctx context.Context, req *RecordListRequest) ([]*storageVO.RecordVO, int64, error)
	GetByID(ctx context.Context, id uint) (*storageVO.RecordVO, error)
	Delete(ctx context.Context, id uint) error
	DeleteMultiple(ctx context.Context, ids []uint) error

	// GetUploadCredentials 签发上传凭证，同时落一条 pending 状态的上传记录，
	// 返回 recordId + secret（HMAC 签名），用于上传成功通知时验签防伪造。
	// source 接收 string 类型（值为 "admin"/"client"/"user"/"api"/"system"），由 handler 端 DTO 常量传入，
	// 避免在 handler 层 import entity/storage 包。实现内部转为 storageEntity.UploadSource 赋值给 entity 字段。
	GetUploadCredentials(ctx context.Context, req *CredentialsRequest, appID string, source string, sourceID string) (*CredentialsResult, error)

	// CompleteUpload 上传成功通知：校验 recordId + secret，通过后将 pending 记录置为 uploaded。
	// 由三套 handler（admin/client/user）统一调用。
	CompleteUpload(ctx context.Context, recordID uint, secret, objectKey, fileURL string, fileSize int64, mimeType, md5 string) (*storageVO.RecordVO, error)
}

type recordService struct {
	recordRepo storageRepo.RecordRepository
	configSvc  ConfigService
	storageMgr *storage.Manager
	appSvc     openSvcPkg.AppService
	tm         database.TxManager
	hmacKey    string // 凭证签名密钥（复用 [jwt] secret）
}

func NewRecordService(
	recordRepo storageRepo.RecordRepository,
	configSvc ConfigService,
	storageMgr *storage.Manager,
	appSvc openSvcPkg.AppService,
	hmacKey string,
	tm database.TxManager,
) RecordService {
	return &recordService{
		recordRepo: recordRepo,
		configSvc:  configSvc,
		storageMgr: storageMgr,
		appSvc:     appSvc,
		tm:         tm,
		hmacKey:    hmacKey,
	}
}

func (s *recordService) List(ctx context.Context, req *RecordListRequest) ([]*storageVO.RecordVO, int64, error) {
	query := &storageRepo.RecordQuery{
		FileName:        req.FileName,
		Source:          req.Source,
		SourceID:        req.SourceID,
		BusinessType:    req.BusinessType,
		BusinessID:      req.BusinessID,
		MimeType:        req.MimeType,
		StorageConfigID: req.StorageConfigID,
		AppID:           req.AppID,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		Current:         req.Current,
		Size:            req.Size,
	}

	query.Current, query.Size = pagination.NormalizePagination(query.Current, query.Size)

	records, total, err := s.recordRepo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	vos := make([]*storageVO.RecordVO, 0, len(records))
	for _, r := range records {
		vos = append(vos, s.toVO(r))
	}

	return vos, total, nil
}

func (s *recordService) GetByID(ctx context.Context, id uint) (*storageVO.RecordVO, error) {
	record, err := s.recordRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "记录不存在")
		}
		slog.Error("recordRepo.GetByID failed", "recordID", id, "err", err)
		return nil, fmt.Errorf("recordRepo.GetByID: %w", err)
	}
	return s.toVO(record), nil
}

func (s *recordService) Delete(ctx context.Context, id uint) error {
	record, err := s.recordRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "记录不存在")
		}
		slog.Error("recordRepo.GetByID failed", "recordID", id, "err", err)
		return fmt.Errorf("recordRepo.GetByID: %w", err)
	}

	driver, err := s.storageMgr.GetDriver(record.StorageConfigID)
	if err == nil {
		if dErr := driver.Delete(ctx, record.FilePath); dErr != nil {
			slog.Warn("delete storage file failed", "recordID", id, "filePath", record.FilePath, "err", dErr)
		}
	}

	return s.recordRepo.Delete(ctx, id)
}

func (s *recordService) DeleteMultiple(ctx context.Context, ids []uint) error {
	for _, id := range ids {
		if err := s.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// GetUploadCredentials 签发上传凭证并落 pending 记录。
func (s *recordService) GetUploadCredentials(ctx context.Context, req *CredentialsRequest, appID string, source string, sourceID string) (*CredentialsResult, error) {
	var config *storageEntity.Config
	var err error

	if appID != "" {
		config, err = s.getAppStorageConfig(ctx, appID, req.ConfigID)
	} else if req.ConfigID > 0 {
		config, err = s.configSvc.GetByID(ctx, req.ConfigID)
	} else {
		config, err = s.configSvc.GetDefault(ctx)
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
		}
		slog.Error("get storage config failed", "appID", appID, "configID", req.ConfigID, "err", err)
		return nil, fmt.Errorf("get storage config: %w", err)
	}

	if !config.IsEnabled() {
		return nil, errorx.New(errorx.CodeForbidden, "存储配置已禁用")
	}

	if config.MaxFileSize > 0 && req.FileSize > config.MaxFileSize {
		return nil, errorx.New(errorx.CodeBadRequest, "文件大小超出限制")
	}

	if config.AllowedTypes != "" {
		if !storage.IsAllowedFileType(req.FileName, config.AllowedTypes) {
			return nil, errorx.New(errorx.CodeBadRequest, "不支持的文件类型")
		}
	}

	driver, err := s.storageMgr.GetDriver(config.ID)
	if err != nil {
		return nil, err
	}

	key := storage.GenerateObjectKeyWithBusiness(req.FileName, config.PathPrefix, req.BusinessType, req.BusinessID)

	contentType := req.ContentType
	if contentType == "" {
		// 上传凭证签发时还没有文件内容，按文件名扩展名推断 MIME
		// （按文件名扩展名推断 MIME 类型，替代原空参 DetectMimeType 调用）
		contentType = storage.MimeTypeByExt(req.FileName)
	}

	expires := 15 * time.Minute
	presignedURL, err := driver.GetPresignedUploadURL(ctx, key, contentType, expires)
	if err != nil {
		return nil, err
	}

	// 统一调用 storage.BuildPublicURL 构造访问 URL（重构清单 B-OTHER-1：
	// 消除原 record.go 中手写 SplitN+TrimSuffix 的 domain 解析逻辑，
	// 与 minio_driver / config.go 共用同一套规范化规则）。
	finalURL := storage.BuildPublicURL(config.Domain, config.Endpoint, config.Bucket, key)

	// 落 pending 上传记录：凭证签发即登记，等待上传成功通知（CompleteUpload）确认状态。
	credExpiresAt := time.Now().Add(credentialTTL)
	record := &storageEntity.Record{
		StorageConfigID: config.ID,
		FileName:        req.FileName,
		StoredName:      req.FileName,
		FilePath:        key, // objectKey，上传成功通知时需校验一致
		FileSize:        req.FileSize,
		MimeType:        contentType,
		FileExt:         storage.GetFileExtension(req.FileName),
		Source:          storageEntity.UploadSource(source),
		SourceID:        sourceID,
		BusinessType:    req.BusinessType,
		BusinessID:      req.BusinessID,
		AppID:           appID,
		Status:          storageEntity.RecordStatusPending,
		ExpiresAt:       &credExpiresAt,
	}
	// TM 单事务原子完成「创建 record + 更新 secret」，任一步失败整体回滚（fail-closed）。
	// 移除补偿 Delete：TM Rollback 自动回滚 Create，无需手动清理。
	// 注意：secret 依赖 record.ID，必须先 Create 拿到 ID 才能签名，故两步均置于事务内。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.recordRepo.Create(txCtx, record); err != nil {
		slog.Error("storage record get credentials: create failed", "err", err)
		s.tm.Rollback(tx)
		return nil, errorx.New(errorx.CodeInternalError, "上传凭证生成失败")
	}
	// 生成 HMAC 签名：recordID|objectKey|source|sourceID|expiresAtUnix
	secret := utils.SignUploadRecord(s.hmacKey, record.ID, key, string(source), sourceID, credExpiresAt.Unix())
	if err := s.recordRepo.UpdateSecret(txCtx, record.ID, secret); err != nil {
		slog.Error("storage record get credentials: update secret failed", "recordID", record.ID, "err", err)
		s.tm.Rollback(tx)
		return nil, errorx.New(errorx.CodeInternalError, "上传凭证生成失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("storage record get credentials: commit failed", "err", err)
		return nil, errorx.New(errorx.CodeInternalError, "上传凭证生成失败")
	}

	return &CredentialsResult{
		URL:         presignedURL,
		Method:      "PUT",
		Headers:     map[string]string{"Content-Type": contentType},
		ExpiresAt:   time.Now().Add(expires),
		ObjectKey:   key,
		Domain:      config.Domain,
		FinalURL:    finalURL,
		ConfigID:    config.ID,
		Region:      config.Region,
		Bucket:      config.Bucket,
		Endpoint:    config.Endpoint,
		PathPrefix:  config.PathPrefix,
		MaxFileSize: config.MaxFileSize,
		RecordID:    record.ID,
		Secret:      secret,
	}, nil
}

// CompleteUpload 上传成功通知的状态机入口。
// 1. 查记录
// 2. 验 HMAC secret（防伪造/防 ID 枚举）
// 3. 验 objectKey 与签名一致（防篡改）
// 4. 验 status == pending（防重复提交/状态错误）
// 5. 验未超期
// 6. LockRecordByID 行锁读取 + FlipStatusToUploaded 条件翻转 status=uploaded（防并发重复提交）
func (s *recordService) CompleteUpload(ctx context.Context, recordID uint, secret, objectKey, fileURL string, fileSize int64, mimeType, md5 string) (*storageVO.RecordVO, error) {
	record, err := s.recordRepo.GetByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeUploadRecordNotFound, "上传记录不存在")
		}
		slog.Error("recordRepo.GetByID failed", "recordID", recordID, "err", err)
		return nil, fmt.Errorf("recordRepo.GetByID: %w", err)
	}

	// 1. secret 为空说明凭证签发异常（如签名写入失败）
	if record.Secret == "" {
		return nil, errorx.New(errorx.CodeUploadSignatureInvalid, "上传凭证校验失败")
	}

	// 2/3. 验签：HMAC(recordID|objectKey|source|sourceID|expiresAt)
	var expiresAtUnix int64
	if record.ExpiresAt != nil {
		expiresAtUnix = record.ExpiresAt.Unix()
	}
	if !utils.VerifyUploadRecord(s.hmacKey, record.ID, record.FilePath, string(record.Source), record.SourceID, expiresAtUnix, secret) {
		return nil, errorx.New(errorx.CodeUploadSignatureInvalid, "上传凭证校验失败")
	}

	// 校验客户端上报的 objectKey 与凭证绑定的 key 一致（防止用 A 凭证给 B 文件登记）
	if objectKey != "" && objectKey != record.FilePath {
		return nil, errorx.New(errorx.CodeUploadRecordMismatch, "上传记录与请求不匹配")
	}

	// 4. 状态机校验
	switch record.Status {
	case storageEntity.RecordStatusUploaded:
		return nil, errorx.New(errorx.CodeUploadRecordCompleted, "该上传记录已完成，不可重复提交")
	case storageEntity.RecordStatusExpired:
		return nil, errorx.New(errorx.CodeUploadRecordExpired, "上传凭证已过期")
	case storageEntity.RecordStatusPending:
		// 正常路径
	default:
		return nil, errorx.New(errorx.CodeUploadRecordMismatch, "上传记录状态异常")
	}

	// 5. 超期校验
	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return nil, errorx.New(errorx.CodeUploadRecordExpired, "上传凭证已过期")
	}

	// 6. 行锁读取 + 条件翻转（并发安全）
	// TM 单事务原子完成「行锁读取 + 条件翻转」，保证 TOCTOU 安全。
	txCtx, tx := s.tm.Begin(ctx)
	locked, err := s.recordRepo.LockRecordByID(txCtx, recordID)
	if err != nil {
		s.tm.Rollback(tx)
		// 行锁读取的 not-found（理论上 GetByID 已挡）转业务错误，避免原始 gorm 错误外泄
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeUploadRecordNotFound, "上传记录不存在")
		}
		return nil, errorx.New(errorx.CodeInternalError, "上传完成处理失败")
	}
	if locked.Status != storageEntity.RecordStatusPending {
		s.tm.Rollback(tx)
		// 行锁下状态已非 pending（并发竞争或状态已变）：按当前状态返回友好错误
		switch locked.Status {
		case storageEntity.RecordStatusUploaded:
			return nil, errorx.New(errorx.CodeUploadRecordCompleted, "该上传记录已完成，不可重复提交")
		case storageEntity.RecordStatusExpired:
			return nil, errorx.New(errorx.CodeUploadRecordExpired, "上传凭证已过期")
		default:
			return nil, errorx.New(errorx.CodeUploadRecordMismatch, "上传记录状态异常")
		}
	}
	updated, err := s.recordRepo.FlipStatusToUploaded(txCtx, recordID, fileSize, md5, mimeType, fileURL)
	if err != nil {
		slog.Error("storage record complete upload: flip status failed", "recordID", recordID, "err", err)
		s.tm.Rollback(tx)
		return nil, errorx.New(errorx.CodeInternalError, "上传完成处理失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("storage record complete upload: commit failed", "recordID", recordID, "err", err)
		return nil, errorx.New(errorx.CodeInternalError, "上传完成处理失败")
	}
	if !updated {
		// TM 事务内行锁保证下，updated=false 不再可能发生（除非并发已翻转）
		// 但保留兜底逻辑以应对边界情况
		fresh, ferr := s.recordRepo.GetByID(ctx, recordID)
		if ferr != nil {
			return nil, errorx.New(errorx.CodeUploadRecordCompleted, "该上传记录已完成，不可重复提交")
		}
		switch fresh.Status {
		case storageEntity.RecordStatusUploaded:
			return nil, errorx.New(errorx.CodeUploadRecordCompleted, "该上传记录已完成，不可重复提交")
		case storageEntity.RecordStatusExpired:
			return nil, errorx.New(errorx.CodeUploadRecordExpired, "上传凭证已过期")
		default:
			return nil, errorx.New(errorx.CodeUploadRecordMismatch, "上传记录状态异常")
		}
	}

	// 重新读取返回完整记录（含更新后的字段与 StorageConfig）
	result, err := s.recordRepo.GetByID(ctx, recordID)
	if err != nil {
		return s.toVO(record), nil
	}
	return s.toVO(result), nil
}

func (s *recordService) toVO(r *storageEntity.Record) *storageVO.RecordVO {
	vo := &storageVO.RecordVO{
		ID:              r.ID,
		StorageConfigID: r.StorageConfigID,
		FileName:        r.FileName,
		StoredName:      r.StoredName,
		FilePath:        r.FilePath,
		FileURL:         r.FileURL,
		FileSize:        r.FileSize,
		MimeType:        r.MimeType,
		FileExt:         r.FileExt,
		MD5:             r.MD5,
		Source:          string(r.Source),
		SourceID:        r.SourceID,
		SourceInfo:      r.SourceInfo,
		UploaderIP:      r.UploaderIP,
		BusinessType:    r.BusinessType,
		BusinessID:      r.BusinessID,
		AppID:           r.AppID,
		Status:          string(r.Status),
		ExpiresAt:       r.ExpiresAt,
		UploadedAt:      r.UploadedAt,
		CreatedAt:       r.CreatedAt,
	}

	if r.StorageConfig != nil {
		vo.StorageName = r.StorageConfig.Name
	}

	return vo
}

func (s *recordService) getAppStorageConfig(ctx context.Context, appID string, fallbackConfigID uint) (*storageEntity.Config, error) {
	app, err := s.appSvc.GetAppByKey(ctx, appID)
	if err == nil && app != nil && app.StorageID > 0 {
		return s.configSvc.GetByID(ctx, app.StorageID)
	}
	if fallbackConfigID > 0 {
		return s.configSvc.GetByID(ctx, fallbackConfigID)
	}
	return s.configSvc.GetDefault(ctx)
}
