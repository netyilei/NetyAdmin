// Package storage 提供对象存储抽象层。
//
// 底层基于 github.com/minio/minio-go/v7 实现，天然兼容所有 S3 协议存储
// （AWS S3 / 阿里云 OSS / 腾讯云 COS / 华为云 OBS / 七牛云 / MinIO / Cloudflare R2 等）。
// 用户在 storage_config 表中直接填写 endpoint 即可，无需项目维护供应商映射表。
package storage

import (
	"context"
	"io"
	"time"
)

// Provider 标识存储服务提供商。
// 仅用于路径风格判断（MinIO/自定义需 path-style）；具体云厂商的供应商清单
// 由 entity 层 StorageProvider 常量维护，避免重复定义。
type Provider string

const (
	// ProviderMinio MinIO 及兼容 S3 的自建存储，需 path-style 寻址。
	ProviderMinio Provider = "minio"
	// ProviderCustom 自定义 endpoint，需 path-style 寻址。
	ProviderCustom Provider = "custom"
)

// IsPathStyle 判断该供应商是否需要使用 path-style 寻址（而非 virtual-host-style）。
// MinIO 与自定义 endpoint 通常不支持 virtual-host-style，需 path-style。
func (p Provider) IsPathStyle() bool {
	switch p {
	case ProviderMinio, ProviderCustom:
		return true
	default:
		return false
	}
}

// Config 存储配置（由 service/storage 层从 entity 转换而来）。
type Config struct {
	ID            uint
	Provider      Provider
	Endpoint      string // 含协议的完整端点，如 https://s3.amazonaws.com 或 https://oss-cn-hangzhou.aliyuncs.com
	Region        string
	Bucket        string
	AccessKey     string
	SecretKey     string
	Domain        string // 自定义 CDN/域名（可选，用于生成访问 URL）
	PathPrefix    string
	IsDefault     bool
	Status        string
	MaxFileSize   int64
	AllowedTypes  string
	STSExpireTime int
}

// IsEnabled 配置是否处于启用状态。
func (c *Config) IsEnabled() bool {
	return c.Status == "1"
}

// UploadResult 上传操作结果。
type UploadResult struct {
	URL      string
	Key      string
	ETag     string
	Size     int64
	MimeType string
}

// ObjectInfo 对象元信息。
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	MimeType     string
}

// Driver 对象存储驱动接口（面向 S3 兼容协议抽象）。
// 当前唯一实现为基于 minio-go 的 minioDriver。
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

// DriverFactory 驱动工厂接口（支持未来扩展多种存储类型）。
type DriverFactory interface {
	Create(config *Config) (Driver, error)
}
