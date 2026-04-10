package storage

import "silentorder/internal/domain/entity"

type StorageProvider string

const (
	StorageProviderAliyun     StorageProvider = "aliyun"
	StorageProviderTencent    StorageProvider = "tencent"
	StorageProviderHuawei     StorageProvider = "huawei"
	StorageProviderQiniu      StorageProvider = "qiniu"
	StorageProviderMinio      StorageProvider = "minio"
	StorageProviderAWS        StorageProvider = "aws"
	StorageProviderCloudflare StorageProvider = "cloudflare"
	StorageProviderCustom     StorageProvider = "custom"
)

type Config struct {
	entity.Model
	entity.Operator
	Name          string          `gorm:"column:name;type:varchar(100);not null;comment:配置名称" json:"name"`
	Provider      StorageProvider `gorm:"column:provider;type:varchar(20);not null;comment:存储提供商" json:"provider"`
	Endpoint      string          `gorm:"column:endpoint;type:varchar(255);not null;comment:服务端点" json:"endpoint"`
	Region        string          `gorm:"column:region;type:varchar(50);comment:区域" json:"region"`
	Bucket        string          `gorm:"column:bucket;type:varchar(100);not null;comment:存储桶名称" json:"bucket"`
	AccessKey     string          `gorm:"column:access_key;type:varchar(255);not null;comment:访问密钥ID" json:"accessKey"`
	SecretKey     string          `gorm:"column:secret_key;type:varchar(255);not null;comment:访问密钥密码" json:"-"`
	Domain        string          `gorm:"column:domain;type:varchar(255);comment:自定义域名" json:"domain"`
	PathPrefix    string          `gorm:"column:path_prefix;type:varchar(100);comment:路径前缀" json:"pathPrefix"`
	IsDefault     bool            `gorm:"column:is_default;default:false;comment:是否默认配置" json:"isDefault"`
	Status        string          `gorm:"column:status;type:char(1);default:'1';comment:状态 1启用 0禁用" json:"status"`
	MaxFileSize   int64           `gorm:"column:max_file_size;default:104857600;comment:最大文件大小(字节) 默认100MB" json:"maxFileSize"`
	AllowedTypes  string          `gorm:"column:allowed_types;type:text;comment:允许的文件类型,逗号分隔" json:"allowedTypes"`
	STSExpireTime int             `gorm:"column:sts_expire_time;default:3600;comment:STS临时凭证过期时间(秒)" json:"stsExpireTime"`
	Remark        string          `gorm:"column:remark;type:text;comment:备注" json:"remark"`
}

func (Config) TableName() string {
	return "storage_config"
}

func (s *Config) IsEnabled() bool {
	return s.Status == "1"
}
