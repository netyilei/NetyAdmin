// driver.go：对象存储驱动契约（精神——S3 通用协议内置驱动
// （minio-go，Apache 2.0，兼容 AWS S3/MinIO/其他 S3 端点）；厂商专用驱动
// 下游按本契约自写 Register 注册）。
package domain

import (
	"context"
	"time"
)

// PresignedPut 是直传凭证（客户端 PUT 到 UploadURL——ObjectKey 即目标位置）。
type PresignedPut struct {
	URL       string
	Method    string
	Headers   map[string]string // 必带请求头（Content-Type 等）
	ExpiresAt time.Time
}

// StorageDriver 是对象存储驱动契约（每配置一实例——endpoint/bucket/凭据绑定）。
type StorageDriver interface {
	// PresignPut 签发直传 PUT URL（expires 内有效；content-type 参与签名）。
	PresignPut(ctx context.Context, objectKey, contentType string, expires time.Duration) (PresignedPut, error)
	// Delete 删除对象（cleanup 面）。
	Delete(ctx context.Context, objectKey string) error
	// PublicURL 对象访问 URL（自定义域名优先——公开读场景）。
	PublicURL(objectKey string) string
}
