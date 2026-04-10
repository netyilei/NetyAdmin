package storage

import (
	"context"
	"strings"

	storageDto "silentorder/internal/interface/admin/dto/storage"
	"silentorder/internal/domain/entity"
	storageEntity "silentorder/internal/domain/entity/storage"
	"silentorder/internal/pkg/errorx"
	"silentorder/internal/pkg/storage"
	storageRepo "silentorder/internal/repository/storage"
)

type ConfigService interface {
	List(ctx context.Context, req *storageDto.ConfigQuery) ([]*storageEntity.Config, int64, error)
	GetByID(ctx context.Context, id uint) (*storageEntity.Config, error)
	Create(ctx context.Context, req *storageDto.CreateConfigReq, operatorID uint) (uint, error)
	Update(ctx context.Context, req *storageDto.UpdateConfigReq, operatorID uint) error
	Delete(ctx context.Context, id uint) error
	SetDefault(ctx context.Context, id uint) error
	TestUpload(ctx context.Context, req *storageDto.TestUploadReq) (string, error)
	LoadAllConfigs(ctx context.Context) error
	GetPresignedUploadURL(ctx context.Context, configID uint, fileName string, contentType string) (string, string, error)
}

type configService struct {
	configRepo storageRepo.ConfigRepository
	recordRepo storageRepo.RecordRepository
	storageMgr *storage.Manager
}

func NewConfigService(
	configRepo storageRepo.ConfigRepository,
	recordRepo storageRepo.RecordRepository,
	storageMgr *storage.Manager,
) ConfigService {
	return &configService{
		configRepo: configRepo,
		recordRepo: recordRepo,
		storageMgr: storageMgr,
	}
}

func (s *configService) List(ctx context.Context, req *storageDto.ConfigQuery) ([]*storageEntity.Config, int64, error) {
	query := &storageRepo.ConfigQuery{
		Current: req.Current,
		Size:    req.Size,
	}

	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = 10
	}

	return s.configRepo.List(ctx, query)
}

func (s *configService) GetByID(ctx context.Context, id uint) (*storageEntity.Config, error) {
	return s.configRepo.GetByID(ctx, id)
}

func (s *configService) Create(ctx context.Context, req *storageDto.CreateConfigReq, operatorID uint) (uint, error) {
	exists, _ := s.configRepo.ExistsByName(ctx, req.Name)
	if exists {
		return 0, errorx.New(errorx.CodeAlreadyExists, "配置名称已存在")
	}

	if req.Provider == "" {
		return 0, errorx.New(errorx.CodeInvalidParams, "存储提供商不能为空")
	}

	provider := storageEntity.StorageProvider(req.Provider)
	if !s.isValidProvider(provider) {
		return 0, errorx.New(errorx.CodeInvalidParams, "不支持的存储提供商")
	}

	if req.Endpoint == "" {
		req.Endpoint = storage.GetProviderEndpoint(storage.Provider(provider), req.Region)
	}

	if req.MaxFileSize <= 0 {
		req.MaxFileSize = 100 * 1024 * 1024
	}

	if req.STSExpireTime <= 0 {
		req.STSExpireTime = 3600
	}

	if req.Status == "" {
		req.Status = "1"
	}

	config := &storageEntity.Config{
		Operator: entity.Operator{
			CreatedBy: operatorID,
			UpdatedBy: operatorID,
		},
		Name:          req.Name,
		Provider:      provider,
		Endpoint:      req.Endpoint,
		Region:        req.Region,
		Bucket:        req.Bucket,
		AccessKey:     req.AccessKey,
		SecretKey:     req.SecretKey,
		Domain:        req.Domain,
		PathPrefix:    req.PathPrefix,
		IsDefault:     req.IsDefault,
		Status:        req.Status,
		MaxFileSize:   req.MaxFileSize,
		AllowedTypes:  req.AllowedTypes,
		STSExpireTime: req.STSExpireTime,
		Remark:        req.Remark,
	}

	if err := s.configRepo.Create(ctx, config); err != nil {
		return 0, err
	}

	if config.IsDefault {
		_ = s.configRepo.SetDefault(ctx, config.ID)
	}

	// 转换为 pkg/storage.Config 进行注册
	pkgConfig := s.toPkgConfig(config)
	if err := s.storageMgr.Register(pkgConfig); err != nil {
		return 0, errorx.New(errorx.CodeInternalError, "存储配置验证失败: "+err.Error())
	}

	return config.ID, nil
}

func (s *configService) Update(ctx context.Context, req *storageDto.UpdateConfigReq, operatorID uint) error {
	config, err := s.configRepo.GetByID(ctx, req.ID)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "配置不存在")
	}

	if config.Name != req.Name {
		exists, _ := s.configRepo.ExistsByName(ctx, req.Name, req.ID)
		if exists {
			return errorx.New(errorx.CodeAlreadyExists, "配置名称已存在")
		}
	}

	provider := storageEntity.StorageProvider(req.Provider)
	if !s.isValidProvider(provider) {
		return errorx.New(errorx.CodeInvalidParams, "不支持的存储提供商")
	}

	oldSecretKey := config.SecretKey
	if req.SecretKey != "" {
		oldSecretKey = req.SecretKey
	}

	config.Name = req.Name
	config.Provider = provider
	config.Endpoint = req.Endpoint
	config.Region = req.Region
	config.Bucket = req.Bucket
	config.AccessKey = req.AccessKey
	if req.SecretKey != "" {
		config.SecretKey = req.SecretKey
	}
	config.Domain = req.Domain
	config.PathPrefix = req.PathPrefix
	config.IsDefault = req.IsDefault
	config.Status = req.Status
	config.MaxFileSize = req.MaxFileSize
	config.AllowedTypes = req.AllowedTypes
	config.STSExpireTime = req.STSExpireTime
	config.Remark = req.Remark
	config.UpdatedBy = operatorID

	if err := s.configRepo.Update(ctx, config); err != nil {
		return err
	}

	if config.IsDefault {
		_ = s.configRepo.SetDefault(ctx, config.ID)
	}

	s.storageMgr.Unregister(config.ID)
	testConfig := *config
	testConfig.SecretKey = oldSecretKey

	if err := s.storageMgr.Register(s.toPkgConfig(&testConfig)); err != nil {
		return errorx.New(errorx.CodeInternalError, "存储配置验证失败: "+err.Error())
	}

	return nil
}

func (s *configService) Delete(ctx context.Context, id uint) error {
	records, err := s.recordRepo.GetByStorageConfigID(ctx, id)
	if err == nil && len(records) > 0 {
		return errorx.New(errorx.CodeBadRequest, "该存储配置下存在上传记录，无法删除")
	}

	if err := s.configRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.storageMgr.Unregister(id)
	return nil
}

func (s *configService) SetDefault(ctx context.Context, id uint) error {
	config, err := s.configRepo.GetByID(ctx, id)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "配置不存在")
	}

	if !config.IsEnabled() {
		return errorx.New(errorx.CodeBadRequest, "只有启用的配置才能设为默认")
	}

	return s.configRepo.SetDefault(ctx, id)
}

func (s *configService) TestUpload(ctx context.Context, req *storageDto.TestUploadReq) (string, error) {
	config, err := s.configRepo.GetByID(ctx, req.ConfigID)
	if err != nil {
		return "", errorx.New(errorx.CodeNotFound, "配置不存在")
	}

	pkgConfig := s.toPkgConfig(config)
	driver, err := storage.NewS3Driver(pkgConfig)
	if err != nil {
		return "", errorx.New(errorx.CodeInternalError, "创建存储驱动失败: "+err.Error())
	}

	key := "test/" + storage.GenerateObjectKey("test.txt", config.PathPrefix)
	content := strings.NewReader(req.Content)

	result, err := driver.Upload(ctx, key, content, int64(len(req.Content)), "text/plain")
	if err != nil {
		return "", errorx.New(errorx.CodeInternalError, "测试上传失败: "+err.Error())
	}

	_ = driver.Delete(ctx, key)

	return result.URL, nil
}

func (s *configService) LoadAllConfigs(ctx context.Context) error {
	configs, err := s.configRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	for _, config := range configs {
		_ = s.storageMgr.Register(s.toPkgConfig(config))
	}

	return nil
}

func (s *configService) GetPresignedUploadURL(ctx context.Context, configID uint, fileName string, contentType string) (string, string, error) {
	config, err := s.configRepo.GetByID(ctx, configID)
	if err != nil {
		return "", "", errorx.New(errorx.CodeNotFound, "配置不存在")
	}

	if !config.IsEnabled() {
		return "", "", errorx.New(errorx.CodeForbidden, "存储配置已禁用")
	}

	if config.AllowedTypes != "" {
		if !storage.IsAllowedFileType(fileName, config.AllowedTypes) {
			return "", "", errorx.New(errorx.CodeBadRequest, "不支持的文件类型")
		}
	}

	driver, err := s.storageMgr.GetDriver(configID)
	if err != nil {
		return "", "", err
	}

	key := storage.GenerateObjectKey(fileName, config.PathPrefix)

	url, err := driver.GetPresignedUploadURL(ctx, key, contentType, 15*60*1000000000)
	if err != nil {
		return "", "", err
	}

	fileURL := config.Domain + "/" + key
	if config.Domain == "" {
		fileURL = "https://" + config.Bucket + "." + strings.TrimPrefix(config.Endpoint, "https://") + "/" + key
	}

	return url, fileURL, nil
}

func (s *configService) isValidProvider(provider storageEntity.StorageProvider) bool {
	validProviders := []storageEntity.StorageProvider{
		storageEntity.StorageProviderAliyun,
		storageEntity.StorageProviderTencent,
		storageEntity.StorageProviderHuawei,
		storageEntity.StorageProviderQiniu,
		storageEntity.StorageProviderMinio,
		storageEntity.StorageProviderAWS,
		storageEntity.StorageProviderCloudflare,
		storageEntity.StorageProviderCustom,
	}
	for _, p := range validProviders {
		if p == provider {
			return true
		}
	}
	return false
}

func (s *configService) toPkgConfig(c *storageEntity.Config) *storage.Config {
	return &storage.Config{
		ID:            c.ID,
		Provider:      storage.Provider(c.Provider),
		Endpoint:      c.Endpoint,
		Region:        c.Region,
		Bucket:        c.Bucket,
		AccessKey:     c.AccessKey,
		SecretKey:     c.SecretKey,
		Domain:        c.Domain,
		PathPrefix:    c.PathPrefix,
		MaxFileSize:   c.MaxFileSize,
		AllowedTypes:  c.AllowedTypes,
		STSExpireTime: c.STSExpireTime,
	}
}
