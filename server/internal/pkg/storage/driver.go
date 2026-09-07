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
	// ProviderCloudflare Cloudflare R2，与 entity 层 StorageProviderCloudflare 同值。
	// R2 端点自带账号子域（<account>.r2.cloudflarestorage.com），
	// 官方推荐 path-style 寻址，virtual-host 不可靠。
	ProviderCloudflare Provider = "cloudflare"
)

// IsPathStyle 判断该供应商是否需要使用 path-style 寻址（而非 virtual-host-style）。
// MinIO、自定义 endpoint 与 Cloudflare R2 的端点不适用（或不可靠）virtual-host-style。
func (p Provider) IsPathStyle() bool {
	switch p {
	case ProviderMinio, ProviderCustom, ProviderCloudflare:
		return true
	default:
		return false
	}
}

// AddressingStyle 桶寻址风格：决定请求 URL 与公开访问 URL 如何拼桶名。
// 请求侧映射为 minio-go 的 BucketLookup 选项，URL 侧决定回退拼接规则，
// 两侧必须由同一决策驱动，否则会出现"上传成功但访问 URL 404"的不一致。
type AddressingStyle int

const (
	// StyleVirtualHost 虚拟主机风格：{scheme}://{bucket}.{host}/{key}。
	// AWS / 阿里云 OSS / 腾讯云 COS / 华为云 OBS / 七牛等主流云厂商的
	// 唯一长期保证寻址方式（AWS 与 OSS 的 path-style 均已官方废弃）。
	StyleVirtualHost AddressingStyle = iota
	// StylePath 路径风格：{scheme}://{host}/{bucket}/{key}。
	// 适用于 MinIO、自建端点与 Cloudflare R2（端点含账号子域）。
	StylePath
)

// AddressingStyleFor 按供应商返回桶寻址风格（请求与 URL 侧的唯一决策点）。
//
// 显式指定而非依赖 minio-go 的 BucketLookupAuto：Auto 仅对 Amazon/Google/阿里云
// 白名单走虚拟主机风格，腾讯 COS 等不在名单内的厂商会错误回落到 path-style，
// 而 COS 只支持虚拟主机寻址。
func AddressingStyleFor(p Provider) AddressingStyle {
	if p.IsPathStyle() {
		return StylePath
	}
	return StyleVirtualHost
}

// Config 存储配置（由 service/storage 层从 entity 转换而来）。
type Config struct {
	ID            uint
	Provider      Provider
	Endpoint      string // 完整端点，如 https://cos.ap-shanghai.myqcloud.com（推荐带协议）；裸 host 为历史遗留形态，按 http 处理
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
//
// 接口仅保留业务实际使用的方法（Round 7 清理）：
//   - Upload / Download：基础上下传
//   - Delete / DeleteMultiple：清理
//   - GetPresignedUploadURL：客户端直传签名（CDN 上传场景核心方法）
//
// 已删除的未使用方法（全代码库 0 调用，且不考虑做后台文件管理）：
//
//	UploadFile / Exists / GetObjectInfo / ListObjects / Copy / GetPresignedDownloadURL
//	—— 文件管理由 CDN 控制台或专用工具完成，不在本系统职责内。
type Driver interface {
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (*UploadResult, error)
	Download(ctx context.Context, key string) (io.ReadCloser, *ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	DeleteMultiple(ctx context.Context, keys []string) error
	GetPresignedUploadURL(ctx context.Context, key string, contentType string, expires time.Duration) (string, error)
}

// DriverFactory 驱动工厂接口（支持未来扩展多种存储类型）。
type DriverFactory interface {
	Create(config *Config) (Driver, error)
}
