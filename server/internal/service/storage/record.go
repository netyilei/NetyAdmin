package storage

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	storageDto "silentorder/internal/interface/admin/dto/storage"
	storageEntity "silentorder/internal/domain/entity/storage"
	storageVO "silentorder/internal/domain/vo/storage"
	"silentorder/internal/pkg/errorx"
	"silentorder/internal/pkg/storage"
	storageRepo "silentorder/internal/repository/storage"
)

type RecordService interface {
	List(ctx context.Context, req *storageDto.RecordQuery) ([]*storageVO.RecordVO, int64, error)
	GetByID(ctx context.Context, id uint) (*storageVO.RecordVO, error)
	Delete(ctx context.Context, id uint) error
	DeleteMultiple(ctx context.Context, ids []uint) error
	CreateRecord(ctx context.Context, record *storageEntity.Record) error
	CreateUploadRecord(ctx context.Context, req *storageDto.CreateRecordReq, source storageEntity.UploadSource, sourceID uint, uploaderIP, userAgent string) (*storageVO.RecordVO, error)
	GetByMD5(ctx context.Context, md5 string) (*storageEntity.Record, error)
	GetUploadCredentials(ctx context.Context, req *storageDto.GetCredentialsReq) (*storageDto.Credentials, error)
	RecordUpload(ctx context.Context, configID uint, fileName, storedName, filePath, fileURL string, fileSize int64, mimeType, md5 string, source storageEntity.UploadSource, sourceID uint, sourceInfo interface{}, uploaderIP, userAgent, businessType string, businessID uint) error
}

type recordService struct {
	recordRepo storageRepo.RecordRepository
	configRepo storageRepo.ConfigRepository
	storageMgr *storage.Manager
}

func NewRecordService(
	recordRepo storageRepo.RecordRepository,
	configRepo storageRepo.ConfigRepository,
	storageMgr *storage.Manager,
) RecordService {
	return &recordService{
		recordRepo: recordRepo,
		configRepo: configRepo,
		storageMgr: storageMgr,
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
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		Current:         req.Current,
		Size:            req.Size,
	}

	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = 10
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

func (s *recordService) CreateRecord(ctx context.Context, record *storageEntity.Record) error {
	return s.recordRepo.Create(ctx, record)
}

func (s *recordService) CreateUploadRecord(ctx context.Context, req *storageDto.CreateRecordReq, source storageEntity.UploadSource, sourceID uint, uploaderIP, userAgent string) (*storageVO.RecordVO, error) {
	var config *storageEntity.Config
	var err error

	if req.ConfigID > 0 {
		config, err = s.configRepo.GetByID(ctx, req.ConfigID)
	} else {
		config, err = s.configRepo.GetDefault(ctx)
	}
	if err != nil {
		return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
	}

	if req.ConfigID == 0 && config != nil {
		req.ConfigID = config.ID
	}

	record := &storageEntity.Record{
		StorageConfigID: req.ConfigID,
		FileName:        req.FileName,
		StoredName:      req.FileName,
		FilePath:        req.ObjectKey,
		FileURL:         "",
		FileSize:        req.FileSize,
		MimeType:        req.MimeType,
		FileExt:         storage.GetFileExtension(req.FileName),
		MD5:             req.MD5,
		Source:          source,
		SourceID:        sourceID,
		SourceInfo:      req.SourceInfo,
		UploaderIP:      uploaderIP,
		UserAgent:       userAgent,
		BusinessType:    req.BusinessType,
		BusinessID:      req.BusinessID,
		UploadedAt:      time.Now(),
	}

	if err := s.recordRepo.Create(ctx, record); err != nil {
		return nil, err
	}

	record.StorageConfig = config
	return s.toVO(record), nil
}

func (s *recordService) GetByMD5(ctx context.Context, md5 string) (*storageEntity.Record, error) {
	return s.recordRepo.GetByMD5(ctx, md5)
}

func (s *recordService) GetUploadCredentials(ctx context.Context, req *storageDto.GetCredentialsReq) (*storageDto.Credentials, error) {
	var config *storageEntity.Config
	var err error

	if req.ConfigID > 0 {
		config, err = s.configRepo.GetByID(ctx, req.ConfigID)
	} else {
		config, err = s.configRepo.GetDefault(ctx)
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

	domainParts := strings.SplitN(config.Domain, "//", 2)
	baseDomain := config.Domain
	if len(domainParts) == 2 {
		hostParts := strings.SplitN(domainParts[1], "/", 2)
		baseDomain = domainParts[0] + "//" + hostParts[0]
	}
	finalURL := strings.TrimSuffix(baseDomain, "/") + "/" + strings.TrimPrefix(key, "/")

	return &storageDto.Credentials{
		URL:             presignedURL,
		Method:          "PUT",
		Headers:         map[string]string{"Content-Type": contentType},
		ExpiresAt:       time.Now().Add(expires),
		ObjectKey:       key,
		Domain:          config.Domain,
		FinalURL:        finalURL,
		ConfigID:        config.ID,
		Region:          storage.GetProviderRegion(storage.Provider(config.Provider), config.Region),
		Bucket:          config.Bucket,
		Endpoint:        config.Endpoint,
		PathPrefix:      config.PathPrefix,
		MaxFileSize:     config.MaxFileSize,
	}, nil
}

func (s *recordService) RecordUpload(ctx context.Context, configID uint, fileName, storedName, filePath, fileURL string, fileSize int64, mimeType, md5 string, source storageEntity.UploadSource, sourceID uint, sourceInfo interface{}, uploaderIP, userAgent, businessType string, businessID uint) error {
	sourceInfoJSON := ""
	if sourceInfo != nil {
		if data, err := json.Marshal(sourceInfo); err == nil {
			sourceInfoJSON = string(data)
		}
	}

	record := &storageEntity.Record{
		StorageConfigID: configID,
		FileName:        fileName,
		StoredName:      storedName,
		FilePath:        filePath,
		FileURL:         fileURL,
		FileSize:        fileSize,
		MimeType:        mimeType,
		FileExt:         storage.GetFileExtension(fileName),
		MD5:             md5,
		Source:          source,
		SourceID:        sourceID,
		SourceInfo:      sourceInfoJSON,
		UploaderIP:      uploaderIP,
		UserAgent:       userAgent,
		BusinessType:    businessType,
		BusinessID:      businessID,
		UploadedAt:      time.Now(),
	}

	return s.recordRepo.Create(ctx, record)
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
		UploadedAt:      r.UploadedAt,
		CreatedAt:       r.CreatedAt,
	}

	if r.StorageConfig != nil {
		vo.StorageName = r.StorageConfig.Name
	}

	return vo
}
