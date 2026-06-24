package storage

import "time"

type RecordQuery struct {
	FileName        string `form:"fileName"`
	Source          string `form:"source"`
	SourceID        string `form:"sourceId"`
	BusinessType    string `form:"businessType"`
	BusinessID      string `form:"businessId"`
	MimeType        string `form:"mimeType"`
	StorageConfigID uint   `form:"storageConfigId"`
	AppID           string `form:"appId"`
	StartTime       string `form:"startTime"`
	EndTime         string `form:"endTime"`
	Current         int    `form:"current"`
	Size            int    `form:"size"`
}

// Credentials 上传凭证响应（含 recordId + secret 用于上传成功通知验签）
type Credentials struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	ExpiresAt   time.Time         `json:"expiresAt"`
	ObjectKey   string            `json:"objectKey"`
	Domain      string            `json:"domain"`
	FinalURL    string            `json:"finalUrl"`
	ConfigID    uint              `json:"configId"`
	Region      string            `json:"region"`
	Bucket      string            `json:"bucket"`
	Endpoint    string            `json:"endpoint"`
	PathPrefix  string            `json:"pathPrefix"`
	MaxFileSize int64             `json:"maxFileSize"`
	RecordID    uint              `json:"recordId"`
	Secret      string            `json:"secret"`
}

type GetCredentialsReq struct {
	ConfigID     uint                   `json:"configId"`
	FileName     string                 `json:"fileName" binding:"required"`
	ContentType  string                 `json:"contentType"`
	FileSize     int64                  `json:"fileSize"`
	BusinessType string                 `json:"businessType"`
	BusinessID   string                 `json:"businessId"`
	SourceInfo   map[string]interface{} `json:"sourceInfo"`
}

// CompleteUploadReq 上传成功通知请求：必须带上凭证签发时返回的 recordId + secret
type CompleteUploadReq struct {
	RecordID  uint   `json:"recordId" binding:"required"`
	Secret    string `json:"secret" binding:"required"`
	ObjectKey string `json:"objectKey"`
	FileURL   string `json:"fileUrl"`
	FileSize  int64  `json:"fileSize"`
	MimeType  string `json:"mimeType"`
	MD5       string `json:"md5"`
}
