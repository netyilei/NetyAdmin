package storage

// UploadSource 上传来源（与 entity/storage.UploadSource 对应，但定义在 DTO 层避免 Handler 引用 entity）
type UploadSource string

const (
	UploadSourceAdmin  UploadSource = "admin"
	UploadSourceClient UploadSource = "client"
	UploadSourceUser   UploadSource = "user"
	UploadSourceAPI    UploadSource = "api"
	UploadSourceSystem UploadSource = "system"
)
