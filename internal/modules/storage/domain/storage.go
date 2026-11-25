// storage.go：对象存储域模型（V1 storage 收敛——S3 通用协议驱动
// 模式（精神：MinIO/AWS S3/兼容端点同一驱动）+ presigned 直传 +
// HMAC complete 防伪）。
package domain

import (
	"errors"
	"strings"
	"time"
)

// 领域错误（调用方映射状态码）。
var (
	ErrConfigNotFound      = errors.New("storage: config not found")
	ErrConfigConflict      = errors.New("storage: default config conflict")
	ErrRecordNotFound      = errors.New("storage: record not found")
	ErrSecretMismatch      = errors.New("storage: credential secret mismatch")
	ErrAlreadyCompleted    = errors.New("storage: record already completed")
	ErrProviderInvalid     = errors.New("storage: provider invalid")
	ErrBucketLookupInvalid = errors.New("storage: bucket lookup invalid")
	ErrPathPrefixInvalid   = errors.New("storage: path prefix invalid")
	ErrDriverFailure       = errors.New("storage: driver failure")
)

// maxPathPrefixLen 是 PathPrefix 长度上限（配置卫生面收敛）。
const maxPathPrefixLen = 64

// ValidatePathPrefix 校验配置 PathPrefix（S1）：仅 [a-zA-Z0-9/_-]、
// 禁 ".."、≤64。objectKey 在 S3 是字面 key 无穿越，但 prefix 参与 PublicURL
// 拼接后客户端 URL 归一化可致路径错位——本校验属配置卫生，非法即拒。
func ValidatePathPrefix(prefix string) error {
	if len(prefix) > maxPathPrefixLen || strings.Contains(prefix, "..") {
		return ErrPathPrefixInvalid
	}
	for _, r := range prefix {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '/', r == '_', r == '-':
		default:
			return ErrPathPrefixInvalid
		}
	}
	return nil
}

// StorageProvider 是存储提供商标识（S3 通用协议——MinIO/AWS S3/兼容端点同驱动）。
type StorageProvider string

const (
	ProviderMinIO StorageProvider = "minio"
	ProviderS3    StorageProvider = "s3"
)

// Valid 校验提供商标识。
func (p StorageProvider) Valid() bool { return p == ProviderMinIO || p == ProviderS3 }

// StorageBucketLookup 是 S3 寻址风格旋钮（ST 联测根修——COS 强制 virtual-host
// 域名 PathStyleDomainForbidden；MinIO/本地=path 默认。机制字段对齐 rclone
// force_path_style / AWS SDK addressing_style 惯例）。
type StorageBucketLookup string

const (
	LookupPath        StorageBucketLookup = "path"
	LookupVirtualHost StorageBucketLookup = "virtual-host"
)

// Valid 校验寻址风格（空=默认 path）。
func (l StorageBucketLookup) Valid() bool {
	return l == "" || l == LookupPath || l == LookupVirtualHost
}

// 记录状态（pending=凭证签发/failed=清理终态/completed=complete 回调）。
const (
	RecordPending   int16 = 1
	RecordCompleted int16 = 2
	RecordExpired   int16 = 3
)

// StorageConfig 是存储配置（V1 config 收敛——S3 通用协议字段面）。
// json tag camelCase（响应面契约—— 教训，大写驼峰即契约漂移）。
type StorageConfig struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Provider     StorageProvider     `json:"provider"`
	Endpoint     string              `json:"endpoint"` // S3 兼容端点（MinIO: http://host:9000）
	Region       string              `json:"region"`
	BucketLookup StorageBucketLookup `json:"bucket_lookup"` // 寻址风格（空=path）
	Bucket       string              `json:"bucket"`
	AccessKey    string              `json:"access_key"`
	SecretKey    string              `json:"secret_key"` // Secret 面（响应掩码，不出域——adapters 注入驱动）
	Domain       string              `json:"domain"`     // 自定义域名（公开读场景 URL 前缀；空=endpoint）
	PathPrefix   string              `json:"path_prefix"`
	IsDefault    bool                `json:"is_default"`
	Status       int16               `json:"status"`        // 1=enabled 2=disabled
	MaxFileSize  int64               `json:"max_file_size"` // 字节；0=不限
	AllowedTypes string              `json:"allowed_types"` // 逗号分隔 MIME；空=不限
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

// Credentials 是上传凭证（签发面——客户端直传后携带 secret complete）。
type Credentials struct {
	RecordID  string            `json:"record_id"`
	UploadURL string            `json:"upload_url"` // presigned PUT
	ObjectKey string            `json:"object_key"`
	Secret    string            `json:"secret"` // HMAC 凭证（complete 校验——防伪造 complete）
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

// UploadRecord 是一次直传记录（pending → completed；过期未 complete → cleanup）。
type UploadRecord struct {
	ID           string     `json:"id"`
	ConfigID     string     `json:"config_id"`
	ObjectKey    string     `json:"object_key"`
	FileName     string     `json:"file_name"`
	FileSize     int64      `json:"file_size"`
	MimeType     string     `json:"mime_type"`
	BusinessType string     `json:"business_type"`
	BusinessID   string     `json:"business_id"`
	Secret       string     `json:"-"` // HMAC 凭证（complete 校验）——出域面关闭：凭证 secret 不进任何响应
	Status       int16      `json:"status"`
	FileURL      string     `json:"file_url"`
	UploaderID   string     `json:"uploader_id"` // authn principal（client 面）
	UploaderIP   string     `json:"uploader_ip"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

// IsExpired 判定 pending 记录是否过凭证有效期（cleanup Job 面）。
func (r *UploadRecord) IsExpired(now time.Time, ttl time.Duration) bool {
	if r.Status != RecordPending {
		return false
	}
	return now.Sub(r.CreatedAt) > ttl
}
