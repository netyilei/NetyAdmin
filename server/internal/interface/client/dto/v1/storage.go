package v1

import "time"

type GetClientCredentialsReq struct {
	FileName     string                 `json:"fileName" binding:"required"`
	ContentType  string                 `json:"contentType"`
	FileSize     int64                  `json:"fileSize"`
	BusinessType string                 `json:"businessType"`
	BusinessID   string                 `json:"businessId"`
	SourceInfo   map[string]interface{} `json:"sourceInfo"`
}

type ClientCredentials struct {
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
}

type CreateClientRecordReq struct {
	FileName        string `json:"fileName" binding:"required"`
	ObjectKey       string `json:"objectKey" binding:"required"`
	FileSize        int64  `json:"fileSize"`
	MimeType        string `json:"mimeType"`
	MD5             string `json:"md5"`
	StorageConfigID uint   `json:"storageConfigId"`
	BusinessType    string `json:"businessType"`
	BusinessID      string `json:"businessId"`
	SourceInfo      string `json:"sourceInfo"`
}
