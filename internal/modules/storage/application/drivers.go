// drivers.go：S3 通用协议驱动（minio-go 实现 domain.StorageDriver——
// 兼容 AWS S3/MinIO/其他 S3 兼容端点；精神「通用协议优先」）。
package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/netyilei/netyadmin/internal/modules/storage/domain"
)

// s3Driver 是 S3 通用协议驱动实现（单配置单实例——ConfigID 绑定）。
type s3Driver struct {
	scheme string // http/https（endpoint scheme）
	host   string // 纯 host（无 scheme——virtual-host 拼接用）
	bucket string
	domain string // 自定义域名（空=endpoint/bucket 拼接）
	lookup minio.BucketLookupType
	client *minio.Client
}

// bucketLookupOf 映射 domain 寻址风格 → minio-go 寻址常量（空/path=默认 path——
// MinIO 兼容；virtual-host=虚拟主机域名——COS 等 S3 兼容服务强制）。
func bucketLookupOf(l domain.StorageBucketLookup) minio.BucketLookupType {
	if l == domain.LookupVirtualHost {
		return minio.BucketLookupDNS
	}
	return minio.BucketLookupPath
}

// newS3Driver 构造 S3 驱动（secure=endpoint 是否 https——由 endpoint scheme 判定）。
func newS3Driver(cfg domain.StorageConfig) (*s3Driver, error) {
	secure := strings.HasPrefix(cfg.Endpoint, "https://")
	host := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "https://"), "http://")
	cli, err := minio.New(host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       secure,
		Region:       cfg.Region,
		BucketLookup: bucketLookupOf(cfg.BucketLookup),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 client: %w", err)
	}
	return &s3Driver{
		scheme: "https", host: host, bucket: cfg.Bucket,
		domain: cfg.Domain, lookup: bucketLookupOf(cfg.BucketLookup), client: cli,
	}, nil
}

// PresignPut 实现 domain.StorageDriver（presigned PUT——Headers 回显约定值）。
func (d *s3Driver) PresignPut(ctx context.Context, objectKey, contentType string, expires time.Duration) (domain.PresignedPut, error) {
	// 注意（B4 语义声明）：minio-go v7 PresignedPutObject 仅签 host——
	// Content-Type 不参与签名，服务端无法强制 PUT 类型；AllowedTypes 是
	// 签发面软策略（签发时校验 MIME 白名单），非传输面强制。
	u, err := d.client.PresignedPutObject(ctx, d.bucket, objectKey, expires)
	if err != nil {
		return domain.PresignedPut{}, fmt.Errorf("storage: presign put: %w", err)
	}
	return domain.PresignedPut{
		URL:     u.String(),
		Method:  "PUT",
		Headers: map[string]string{"Content-Type": contentType},
		// UTC 显式（S3 SigV4 签名过期时刻即 UTC——本地时区直加会漂移过期窗口）
		ExpiresAt: time.Now().UTC().Add(expires),
	}, nil
}

// Delete 实现 domain.StorageDriver（cleanup Job 孤儿对象删除消费——B3）。
func (d *s3Driver) Delete(ctx context.Context, objectKey string) error {
	return d.client.RemoveObject(ctx, d.bucket, objectKey, minio.RemoveObjectOptions{})
}

// PublicURL 实现 domain.StorageDriver（自定义域名优先——域名含路径前缀由
// 配置方拼接；无域名按寻址风格拼：virtual-host=<bucket>.<host>/key、
// path=<endpoint>/<bucket>/key——B7 修正：virtual-host 配置下原恒 path 形态）。
func (d *s3Driver) PublicURL(objectKey string) string {
	if d.domain != "" {
		return strings.TrimSuffix(d.domain, "/") + "/" + objectKey
	}
	if d.lookup == minio.BucketLookupDNS {
		return d.scheme + "://" + d.bucket + "." + d.host + "/" + objectKey
	}
	return d.scheme + "://" + d.host + "/" + d.bucket + "/" + objectKey
}
