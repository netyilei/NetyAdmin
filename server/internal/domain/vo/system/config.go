package system

type SysConfigVO struct {
	GroupName   string `json:"groupName"`
	ConfigKey   string `json:"configKey"`
	ConfigValue string `json:"configValue"`
	ValueType   string `json:"valueType"`
	Description string `json:"description"`
	IsSystem    bool   `json:"isSystem"`
}
