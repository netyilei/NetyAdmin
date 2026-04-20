package system

type ConfigQuery struct {
	GroupName string `form:"groupName"`
}

// UpdateConfigReq 我们采用了 Upsert（或专门更新配置项）的形式
type UpdateConfigReq struct {
	GroupName   string `json:"groupName" binding:"required"`
	ConfigKey   string `json:"configKey" binding:"required"`
	ConfigValue string `json:"configValue"`
	ValueType   string `json:"valueType"`
	Description string `json:"description"`
}

// 供快捷开关缓存用
type ToggleCacheReq struct {
	ModuleName string `json:"moduleName" binding:"required"`
	Enabled    bool   `json:"enabled"`
}

type TestEmailReq struct {
	Receiver string `json:"receiver" binding:"required,email"`
}
