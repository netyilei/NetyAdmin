package v1

import "time"

// UploadSource 上传来源（与 entity/storage.UploadSource 对应，但定义在 DTO 层避免 Handler 引用 entity）
type UploadSource string

const (
	UploadSourceAdmin  UploadSource = "admin"
	UploadSourceClient UploadSource = "client"
	UploadSourceUser   UploadSource = "user"
	UploadSourceAPI    UploadSource = "api"
	UploadSourceSystem UploadSource = "system"
)

type GetClientCredentialsReq struct {
	FileName     string                 `json:"fileName" binding:"required"`
	ContentType  string                 `json:"contentType"`
	FileSize     int64                  `json:"fileSize"`
	BusinessType string                 `json:"businessType"`
	BusinessID   string                 `json:"businessId"`
	SourceInfo   map[string]interface{} `json:"sourceInfo"`
}

// ClientCredentials 上传凭证响应（含 recordId + secret 用于上传成功通知验签）
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
	RecordID    uint              `json:"recordId"`
	Secret      string            `json:"secret"`
}

// CompleteClientUploadReq 上传成功通知请求：必须带上凭证签发时返回的 recordId + secret
type CompleteClientUploadReq struct {
	RecordID  uint   `json:"recordId" binding:"required"`
	Secret    string `json:"secret" binding:"required"`
	ObjectKey string `json:"objectKey"`
	FileURL   string `json:"fileUrl"`
	FileSize  int64  `json:"fileSize"`
	MimeType  string `json:"mimeType"`
	MD5       string `json:"md5"`
}
