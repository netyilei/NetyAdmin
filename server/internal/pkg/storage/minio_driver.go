package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// minioDriver 基于 minio-go 的 S3 兼容存储驱动。
// 支持所有 S3 协议存储（AWS S3 / 阿里云 OSS / 腾讯云 COS / 华为云 OBS / MinIO 等）。
type minioDriver struct {
	client     *minio.Client
	bucket     string
	domain     string
	pathPrefix string
}

// NewMinioDriver 创建 minio-go 驱动实例。
//
// Endpoint 应包含协议前缀（https:// 或 http://），minio-go 会据此判断是否启用 TLS。
// 对于 MinIO / 自建 S3 兼容存储（Provider 为 minio/custom），使用 path-style 寻址。
func NewMinioDriver(cfg *Config) (Driver, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("endpoint 不能为空")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("bucket 不能为空")
	}

	// minio-go 的 New 接受含协议的 endpoint，自动解析 TLS 与 host。
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       isTLSEndpoint(cfg.Endpoint),
		Region:       cfg.Region,
		BucketLookup: bucketLookupType(cfg.Provider),
	})
	if err != nil {
		return nil, fmt.Errorf("创建 minio 客户端失败: %w", err)
	}

	return &minioDriver{
		client:     client,
		bucket:     cfg.Bucket,
		domain:     cfg.Domain,
		pathPrefix: cfg.PathPrefix,
	}, nil
}

// isTLSEndpoint 根据 endpoint 的协议前缀判断是否启用 TLS。
func isTLSEndpoint(endpoint string) bool {
	return strings.HasPrefix(endpoint, "https://")
}

// bucketLookupType 根据供应商选择寻址方式。
func bucketLookupType(p Provider) minio.BucketLookupType {
	if p.IsPathStyle() {
		return minio.BucketLookupPath
	}
	return minio.BucketLookupAuto
}

// buildKey 拼接路径前缀（确保所有对象统一存放于配置的子目录下）。
func (d *minioDriver) buildKey(key string) string {
	if d.pathPrefix != "" && !strings.HasPrefix(key, d.pathPrefix+"/") {
		return d.pathPrefix + "/" + key
	}
	return key
}

// buildURL 构造对象的访问 URL。
//
// 委托给 storage.BuildPublicURL 的核心规则（domain 规范化 + endpoint 回退），
// 保持 minioDriver 与 record.go / config.go 的 URL 生成逻辑一致（重构清单 B-OTHER-1）。
// minioDriver 已在构造时拆解 Config，此处复用同一套规范化逻辑而非重新实现。
func (d *minioDriver) buildURL(key string) string {
	key = strings.TrimPrefix(key, "/")
	if d.domain != "" {
		return joinDomainKey(normalizeDomain(d.domain), key)
	}
	// 回退到 client endpoint（已含协议）
	return strings.TrimSuffix(d.client.EndpointURL().String(), "/") + "/" + d.bucket + "/" + key
}

// Upload 上传对象（流式）。
func (d *minioDriver) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (*UploadResult, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fullKey := d.buildKey(key)

	info, err := d.client.PutObject(ctx, d.bucket, fullKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("上传对象失败: %w", err)
	}

	return &UploadResult{
		URL:      d.buildURL(fullKey),
		Key:      fullKey,
		ETag:     strings.Trim(info.ETag, "\""),
		Size:     info.Size,
		MimeType: contentType,
	}, nil
}

// Download 下载对象，返回可读流与对象元信息。
func (d *minioDriver) Download(ctx context.Context, key string) (io.ReadCloser, *ObjectInfo, error) {
	fullKey := d.buildKey(key)

	obj, err := d.client.GetObject(ctx, d.bucket, fullKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("下载对象失败: %w", err)
	}

	// GetObject 返回的 *minio.Object 需要读取 Stat 才能拿到元信息。
	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, fmt.Errorf("获取对象元信息失败: %w", err)
	}

	info := toObjectInfo(fullKey, stat)
	return obj, info, nil
}

// Delete 删除单个对象。
func (d *minioDriver) Delete(ctx context.Context, key string) error {
	fullKey := d.buildKey(key)
	if err := d.client.RemoveObject(ctx, d.bucket, fullKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("删除对象失败: %w", err)
	}
	return nil
}

// DeleteMultiple 批量删除对象。
func (d *minioDriver) DeleteMultiple(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	// minio-go 的 RemoveObjects 通过 channel 接收待删除对象名。
	objCh := make(chan minio.ObjectInfo, len(keys))
	for _, key := range keys {
		objCh <- minio.ObjectInfo{Key: d.buildKey(key)}
	}
	close(objCh)

	errCh := d.client.RemoveObjects(ctx, d.bucket, objCh, minio.RemoveObjectsOptions{})
	var errMsgs []string
	for err := range errCh {
		errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", err.ObjectName, err.Err.Error()))
	}
	if len(errMsgs) > 0 {
		return fmt.Errorf("部分删除失败: %s", strings.Join(errMsgs, "; "))
	}
	return nil
}

// GetPresignedUploadURL 生成预签名上传 URL（PUT）。
func (d *minioDriver) GetPresignedUploadURL(ctx context.Context, key string, _ string, expires time.Duration) (string, error) {
	if expires == 0 {
		expires = defaultPresignExpiry
	}
	u, err := d.client.PresignedPutObject(ctx, d.bucket, d.buildKey(key), expires)
	if err != nil {
		return "", fmt.Errorf("生成预签名上传URL失败: %w", err)
	}
	return u.String(), nil
}

// toObjectInfo 将 minio.ObjectInfo 转换为本包的 ObjectInfo。
func toObjectInfo(key string, stat minio.ObjectInfo) *ObjectInfo {
	return &ObjectInfo{
		Key:          key,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		ETag:         strings.Trim(stat.ETag, "\""),
		MimeType:     stat.ContentType,
	}
}

// MimeTypeByExt 根据文件名扩展名推断 MIME 类型。
//
// 适用于上传凭证签发场景（此时还没有文件内容，只有文件名），
// 按文件名扩展名推断 MIME 类型，替代原 record.go 中空参 DetectMimeType 调用。
// 扩展名未命中映射表时返回 application/octet-stream。
func MimeTypeByExt(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if mime, ok := extensionMimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// DetectMimeType 通过文件内容嗅探 MIME 类型。
// 仅在已有文件内容时使用（如 CompleteUpload 后续处理）。
// 上传凭证签发场景（仅有文件名）应使用 MimeTypeByExt。
func DetectMimeType(data []byte) string {
	return http.DetectContentType(data)
}

// extensionMimeTypes 常见文件扩展名到 MIME 的映射。
var extensionMimeTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".mp4":  "video/mp4",
	".mp3":  "audio/mpeg",
	".zip":  "application/zip",
	".json": "application/json",
	".xml":  "application/xml",
	".txt":  "text/plain",
	".html": "text/html",
	".css":  "text/css",
	".js":   "application/javascript",
}

// MinioDriverFactory minio-go 驱动工厂实现。
type MinioDriverFactory struct{}

// Create 实现 DriverFactory 接口。
func (f *MinioDriverFactory) Create(config *Config) (Driver, error) {
	return NewMinioDriver(config)
}

// NewMinioDriverFactory 创建工厂实例。
func NewMinioDriverFactory() *MinioDriverFactory {
	return &MinioDriverFactory{}
}

// 默认值常量（避免魔法数字）。
const (
	// defaultPresignExpiry 预签名 URL 默认有效期。
	defaultPresignExpiry = 15 * time.Minute
)
