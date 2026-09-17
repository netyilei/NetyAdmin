package utils

import "time"

// ParseRFC3339 解析 RFC3339（带时区偏移）时间字符串。
// 各 service 时间字段的统一解析入口（原先 article/banner 等各自内联
// time.Parse(time.RFC3339, ...)），报错文案由调用方给出业务语境。
func ParseRFC3339(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
