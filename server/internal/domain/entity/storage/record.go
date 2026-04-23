package storage

import (
	"time"

	"NetyAdmin/internal/domain/entity"
)

type UploadSource string

const (
	UploadSourceAdmin  UploadSource = "admin"
	UploadSourceClient UploadSource = "client"
	UploadSourceAPI    UploadSource = "api"
	UploadSourceSystem UploadSource = "system"
)

type Record struct {
	entity.Model
	StorageConfigID uint         `gorm:"column:storage_config_id;not null;index;comment:存储配置ID" json:"storageConfigId"`
	StorageConfig   *Config      `gorm:"foreignKey:StorageConfigID;references:ID" json:"storageConfig"`
	FileName        string       `gorm:"column:file_name;type:varchar(255);not null;comment:原始文件名" json:"fileName"`
	StoredName      string       `gorm:"column:stored_name;type:varchar(255);not null;comment:存储文件名" json:"storedName"`
	FilePath        string       `gorm:"column:file_path;type:varchar(500);not null;comment:文件路径" json:"filePath"`
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
	UploadedAt      time.Time    `gorm:"column:uploaded_at;autoCreateTime;comment:上传时间" json:"uploadedAt"`
}

func (Record) TableName() string {
	return "upload_record"
}

func (r *Record) IsImage() bool {
	imageTypes := []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/svg+xml"}
	for _, t := range imageTypes {
		if r.MimeType == t {
			return true
		}
	}
	return false
}

func (r *Record) IsVideo() bool {
	videoTypes := []string{"video/mp4", "video/mpeg", "video/quicktime", "video/x-msvideo", "video/webm"}
	for _, t := range videoTypes {
		if r.MimeType == t {
			return true
		}
	}
	return false
}

func (r *Record) IsAudio() bool {
	audioTypes := []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/aac", "audio/flac"}
	for _, t := range audioTypes {
		if r.MimeType == t {
			return true
		}
	}
	return false
}
