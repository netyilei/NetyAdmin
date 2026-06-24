package storage

import (
	"time"

	"NetyAdmin/internal/domain/entity"
)

type UploadSource string

const (
	UploadSourceAdmin  UploadSource = "admin"
	UploadSourceClient UploadSource = "client"
	UploadSourceUser   UploadSource = "user"
	UploadSourceAPI    UploadSource = "api"
	UploadSourceSystem UploadSource = "system"
)

// RecordStatus 上传记录状态机
type RecordStatus string

const (
	// RecordStatusPending 待上传：凭证已签发，等待上传成功通知
	RecordStatusPending RecordStatus = "pending"
	// RecordStatusUploaded 已上传：上传成功通知验签通过
	RecordStatusUploaded RecordStatus = "uploaded"
	// RecordStatusExpired 已过期：凭证签发后超期未通知，定时任务兜底标记
	RecordStatusExpired RecordStatus = "expired"
)

type Record struct {
	entity.Model
	StorageConfigID uint         `gorm:"column:storage_config_id;not null;index;comment:存储配置ID" json:"storageConfigId"`
	StorageConfig   *Config      `gorm:"foreignKey:StorageConfigID;references:ID" json:"storageConfig"`
	FileName        string       `gorm:"column:file_name;type:varchar(255);not null;comment:原始文件名" json:"fileName"`
	StoredName      string       `gorm:"column:stored_name;type:varchar(255);not null;comment:存储文件名" json:"storedName"`
	FilePath        string       `gorm:"column:file_path;type:varchar(500);not null;comment:文件路径(objectKey)" json:"filePath"`
	FileURL         string       `gorm:"column:file_url;type:varchar(500);comment:文件访问URL" json:"fileUrl"`
	FileSize        int64        `gorm:"column:file_size;not null;comment:文件大小(字节)" json:"fileSize"`
	MimeType        string       `gorm:"column:mime_type;type:varchar(100);comment:文件MIME类型" json:"mimeType"`
	FileExt         string       `gorm:"column:file_ext;type:varchar(20);comment:文件扩展名" json:"fileExt"`
	MD5             string       `gorm:"column:md5;type:varchar(32);comment:文件MD5" json:"md5"`
	Source          UploadSource `gorm:"column:source;type:varchar(20);not null;index;comment:上传来源" json:"source"`
	SourceID        string       `gorm:"column:source_id;size:26;index;comment:来源ID(如管理员ID/用户ID)" json:"sourceId"`
	SourceInfo      string       `gorm:"column:source_info;type:text;comment:来源附加信息JSON" json:"sourceInfo"`
	UploaderIP      string       `gorm:"column:uploader_ip;type:varchar(50);comment:上传者IP" json:"uploaderIp"`
	UserAgent       string       `gorm:"column:user_agent;type:varchar(500);comment:用户代理" json:"userAgent"`
	BusinessType    string       `gorm:"column:business_type;type:varchar(50);index;comment:业务类型" json:"businessType"`
	BusinessID      string       `gorm:"column:business_id;size:26;index;comment:业务ID" json:"businessId"`
	AppID           string       `gorm:"column:app_id;size:26;index;comment:开放平台应用ID" json:"appId"`
	// Status 上传状态：pending/uploaded/expired。注意不设默认值 tag，
	// 由迁移脚本的列默认值 'uploaded' 兜底历史数据；新记录由 service 显式赋值。
	Status     RecordStatus `gorm:"column:status;type:varchar(20);not null;index;comment:上传状态" json:"status"`
	Secret     string       `gorm:"column:secret;type:varchar(128);not null;default:'';comment:凭证HMAC签名" json:"-"`
	ExpiresAt  *time.Time   `gorm:"column:expires_at;comment:凭证过期时间" json:"expiresAt"`
	UploadedAt *time.Time   `gorm:"column:uploaded_at;comment:上传完成时间" json:"uploadedAt"`
}

func (Record) TableName() string {
	return "upload_record"
}

var (
	imageMimeTypes = map[string]bool{
		"image/jpeg": true, "image/png": true, "image/gif": true,
		"image/webp": true, "image/bmp": true, "image/svg+xml": true,
	}
	videoMimeTypes = map[string]bool{
		"video/mp4": true, "video/mpeg": true, "video/quicktime": true,
		"video/x-msvideo": true, "video/webm": true,
	}
	audioMimeTypes = map[string]bool{
		"audio/mpeg": true, "audio/wav": true, "audio/ogg": true,
		"audio/aac": true, "audio/flac": true,
	}
)

func (r *Record) IsImage() bool {
	return imageMimeTypes[r.MimeType]
}

func (r *Record) IsVideo() bool {
	return videoMimeTypes[r.MimeType]
}

func (r *Record) IsAudio() bool {
	return audioMimeTypes[r.MimeType]
}
