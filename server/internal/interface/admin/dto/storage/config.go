package storage

type ConfigQuery struct {
	Current int `form:"current"`
	Size    int `form:"size"`
}

type CreateConfigReq struct {
	Name          string `json:"name" binding:"required"`
	Provider      string `json:"provider" binding:"required"`
	Endpoint      string `json:"endpoint" binding:"required"`
	Region        string `json:"region"`
	Bucket        string `json:"bucket" binding:"required"`
	AccessKey     string `json:"accessKey" binding:"required"`
	SecretKey     string `json:"secretKey" binding:"required"`
	Domain        string `json:"domain"`
	PathPrefix    string `json:"pathPrefix"`
	IsDefault     bool   `json:"isDefault"`
	Status        string `json:"status"`
	MaxFileSize   int64  `json:"maxFileSize"`
	AllowedTypes  string `json:"allowedTypes"`
	STSExpireTime int    `json:"stsExpireTime"`
	Remark        string `json:"remark"`
}

type UpdateConfigReq struct {
	ID            uint   `json:"id" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Provider      string `json:"provider" binding:"required"`
	Endpoint      string `json:"endpoint" binding:"required"`
	Region        string `json:"region"`
	Bucket        string `json:"bucket" binding:"required"`
	AccessKey     string `json:"accessKey"`
	SecretKey     string `json:"secretKey"`
	Domain        string `json:"domain"`
	PathPrefix    string `json:"pathPrefix"`
	IsDefault     bool   `json:"isDefault"`
	Status        string `json:"status"`
	MaxFileSize   int64  `json:"maxFileSize"`
	AllowedTypes  string `json:"allowedTypes"`
	STSExpireTime int    `json:"stsExpireTime"`
	Remark        string `json:"remark"`
}

type TestUploadReq struct {
	ConfigID uint   `json:"configId" binding:"required"`
	FileName string `json:"fileName" binding:"required"`
	Content  string `json:"content" binding:"required"`
}
