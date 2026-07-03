package storage

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	storageEntity "NetyAdmin/internal/domain/entity/storage"
	storageVO "NetyAdmin/internal/domain/vo/storage"
	storageDto "NetyAdmin/internal/interface/admin/dto/storage"
	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/storage"
	"NetyAdmin/internal/pkg/utils"
	storageRepo "NetyAdmin/internal/repository/storage"
	openSvcPkg "NetyAdmin/internal/service/open_platform"
)

// credentialTTL 凭证（pending 记录）有效期：超过此时间未收到上传成功通知则视为过期。
const credentialTTL = 30 * time.Minute

type RecordService interface {
	List(ctx context.Context, req *storageDto.RecordQuery) ([]*storageVO.RecordVO, int64, error)
	GetByID(ctx context.Context, id uint) (*storageVO.RecordVO, error)
	Delete(ctx context.Context, id uint) error
	DeleteMultiple(ctx context.Context, ids []uint) error

	// GetUploadCredentials 签发上传凭证，同时落一条 pending 状态的上传记录，
	// 返回 recordId + secret（HMAC 签名），用于上传成功通知时验签防伪造。
	GetUploadCredentials(ctx context.Context, req *storageDto.GetCredentialsReq, appID string, source storageEntity.UploadSource, sourceID string) (*storageDto.Credentials, error)

	// CompleteUpload 上传成功通知：校验 recordId + secret，通过后将 pending 记录置为 uploaded。
	// 由三套 handler（admin/client/user）统一调用。
	CompleteUpload(ctx context.Context, recordID uint, secret, objectKey, fileURL string, fileSize int64, mimeType, md5 string) (*storageVO.RecordVO, error)
}

type recordService struct {
	recordRepo storageRepo.RecordRepository
	configSvc  ConfigService
	storageMgr *storage.Manager
	appSvc     openSvcPkg.AppService
	hmacKey    string // 凭证签名密钥（复用 [jwt] secret）
}

func NewRecordService(
	recordRepo storageRepo.RecordRepository,
	configSvc ConfigService,
	storageMgr *storage.Manager,
	appSvc openSvcPkg.AppService,
	hmacKey string,
) RecordService {
	return &recordService{
		recordRepo: recordRepo,
		configSvc:  configSvc,
		storageMgr: storageMgr,
		appSvc:     appSvc,
		hmacKey:    hmacKey,
	}
}

func (s *recordService) List(ctx context.Context, req *storageDto.RecordQuery) ([]*storageVO.RecordVO, int64, error) {
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

	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = entity.DefaultPageSize
	}

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
		return nil, errorx.New(errorx.CodeNotFound, "记录不存在")
	}
	return s.toVO(record), nil
}

func (s *recordService) Delete(ctx context.Context, id uint) error {
	record, err := s.recordRepo.GetByID(ctx, id)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "记录不存在")
	}

	driver, err := s.storageMgr.GetDriver(record.StorageConfigID)
	if err == nil {
		_ = driver.Delete(ctx, record.FilePath)
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
func (s *recordService) GetUploadCredentials(ctx context.Context, req *storageDto.GetCredentialsReq, appID string, source storageEntity.UploadSource, sourceID string) (*storageDto.Credentials, error) {
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
		return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
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
		contentType = storage.DetectMimeType([]byte{})
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
		Source:          source,
		SourceID:        sourceID,
		BusinessType:    req.BusinessType,
		BusinessID:      req.BusinessID,
		AppID:           appID,
		Status:          storageEntity.RecordStatusPending,
		ExpiresAt:       &credExpiresAt,
	}
	if err := s.recordRepo.Create(ctx, record); err != nil {
		return nil, err
	}

	// 生成 HMAC 签名：recordID|objectKey|source|sourceID|expiresAtUnix
	secret := utils.SignUploadRecord(s.hmacKey, record.ID, key, string(source), sourceID, credExpiresAt.Unix())
	// 写回 secret：recordID 在 Create 后才生成，签名依赖 ID，故需二次写入。
	if err := s.recordRepo.UpdateSecret(ctx, record.ID, secret); err != nil {
		// 写入 secret 失败：回滚（删除刚创建的 pending 记录），避免下发无签名凭证
		_ = s.recordRepo.Delete(ctx, record.ID)
		return nil, err
	}

	return &storageDto.Credentials{
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
// 6. MarkUploaded：事务内行锁 + 条件翻转 status=uploaded（防并发重复提交）
func (s *recordService) CompleteUpload(ctx context.Context, recordID uint, secret, objectKey, fileURL string, fileSize int64, mimeType, md5 string) (*storageVO.RecordVO, error) {
	record, err := s.recordRepo.GetByID(ctx, recordID)
	if err != nil {
		return nil, errorx.New(errorx.CodeUploadRecordNotFound, "上传记录不存在")
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

	// 6. 事务内行锁 + 条件翻转（并发安全）
	updated, err := s.recordRepo.MarkUploaded(ctx, recordID, fileSize, md5, mimeType, fileURL)
	if err != nil {
		// 事务内的 not-found（理论上 GetByID 已挡）转业务错误，避免原始 gorm 错误外泄
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeUploadRecordNotFound, "上传记录不存在")
		}
		return nil, err
	}
	if !updated {
		// 并发竞争：记录已被另一个请求翻转，按当前状态返回友好错误
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
