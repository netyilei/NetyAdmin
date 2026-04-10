package storage

import (
	"context"
	"io"
	"time"
)

type Provider string

const (
	ProviderAliyun     Provider = "aliyun"
	ProviderTencent    Provider = "tencent"
	ProviderHuawei     Provider = "huawei"
	ProviderQiniu      Provider = "qiniu"
	ProviderMinio      Provider = "minio"
	ProviderAWS        Provider = "aws"
	ProviderCloudflare Provider = "cloudflare"
	ProviderCustom     Provider = "custom"
)

type Config struct {
	ID            uint
	Provider      Provider
	Endpoint      string
	Region        string
	Bucket        string
	AccessKey     string
	SecretKey     string
	Domain        string
	PathPrefix    string
	IsDefault     bool
	Status        string
	MaxFileSize   int64
	AllowedTypes  string
	STSExpireTime int
}

func (c *Config) IsEnabled() bool {
	return c.Status == "1"
}

type UploadResult struct {
	URL      string
	Key      string
	ETag     string
	Size     int64
	MimeType string
}

type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	MimeType     string
}

type Driver interface {
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (*UploadResult, error)
	UploadFile(ctx context.Context, key string, filePath string, contentType string) (*UploadResult, error)
	Download(ctx context.Context, key string) (io.ReadCloser, *ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	DeleteMultiple(ctx context.Context, keys []string) error
	Exists(ctx context.Context, key string) (bool, error)
	GetObjectInfo(ctx context.Context, key string) (*ObjectInfo, error)
	GetPresignedUploadURL(ctx context.Context, key string, contentType string, expires time.Duration) (string, error)
	GetPresignedDownloadURL(ctx context.Context, key string, expires time.Duration) (string, error)
	ListObjects(ctx context.Context, prefix string, maxKeys int) ([]*ObjectInfo, error)
	Copy(ctx context.Context, srcKey, destKey string) error
}

type DriverFactory interface {
	Create(config *Config) (Driver, error)
}

func GetProviderEndpoint(provider Provider, region string) string {
	switch provider {
	case ProviderAliyun:
		if region == "" {
			region = "oss-cn-hangzhou"
		}
		return "https://oss-" + region + ".aliyuncs.com"
	case ProviderTencent:
		if region == "" {
			region = "ap-guangzhou"
		}
		return "https://cos." + region + ".myqcloud.com"
	case ProviderHuawei:
		if region == "" {
			region = "cn-north-4"
		}
		return "https://obs." + region + ".myhuaweicloud.com"
	case ProviderQiniu:
		return "https://s3-cn-south-1.qiniucs.com"
	case ProviderAWS:
		if region == "" {
			region = "us-east-1"
		}
		return "https://s3." + region + ".amazonaws.com"
	case ProviderCloudflare:
		return "https://" + region + ".r2.cloudflarestorage.com"
	default:
		return ""
	}
}

func GetProviderRegion(provider Provider, configRegion string) string {
	if configRegion != "" {
		return configRegion
	}
	switch provider {
	case ProviderAliyun:
		return "oss-cn-hangzhou"
	case ProviderTencent:
		return "ap-guangzhou"
	case ProviderHuawei:
		return "cn-north-4"
	case ProviderQiniu:
		return "cn-south-1"
	case ProviderAWS:
		return "us-east-1"
	case ProviderMinio:
		return "us-east-1"
	case ProviderCloudflare:
		return "auto"
	default:
		return "us-east-1"
	}
}
