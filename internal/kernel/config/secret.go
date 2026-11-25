package config

import "strings"

// Secret 是密钥类配置值：只从环境变量注入（security.md §1），不入配置文件。
// 不可变；IsPlaceholder 用于生产 fail-closed 校验。
type Secret struct {
	value  string
	isSet  bool
	source string
}

// NewSecret 构造一个已注入的密钥值。
func NewSecret(value, source string) Secret {
	return Secret{value: value, isSet: true, source: source}
}

// IsSet 密钥是否已注入（未注入 = 空）。
func (s Secret) IsSet() bool { return s.isSet }

// IsPlaceholder 是否为占位符/明显未配置（生产启动必须 fail-closed，security.md §1）。
func (s Secret) IsPlaceholder() bool {
	if !s.isSet {
		return true
	}
	v := strings.ToUpper(strings.TrimSpace(s.value))
	if v == "" {
		return true
	}
	for _, marker := range []string{"CHANGE_ME", "PLACEHOLDER", "REPLACE_ME", "TODO", "<"} {
		if strings.Contains(v, marker) {
			return true
		}
	}
	return false
}

// String 实现 fmt.Stringer：脱敏输出，禁止取得原值落日志。
func (s Secret) String() string { return "[redacted]" }

// GoString 实现 fmt.GoStringer：堵住 %#v 打印未导出字段（密钥原值）的绕过路径。
func (s Secret) GoString() string { return "[redacted]" }

// Value 返回密钥原值——仅限基础设施装配场景（如 DSN 构建），
// 严禁用于日志/导出/信封（脱敏面由 String/GoString 保证）。
func (s Secret) Value() string { return s.value }
