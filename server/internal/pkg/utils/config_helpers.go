package utils

import (
	"strconv"

	"NetyAdmin/internal/pkg/configsync"
)

// GetIntWithDefault 从 ConfigWatcher 读取字符串配置并转 int，失败或 <=0 时返回默认值。
//
// 抽取自 logbus.parseInt / open_platform.getDefaultRate / common_handler 的
// 分散 strconv.Atoi + fallback 模式（重构清单 B-OTHER-5）。
// 统一行为：
//   - 配置不存在 / 非数字 / <=0 → 返回 defaultVal
//   - 否则返回解析值
func GetIntWithDefault(watcher configsync.ConfigWatcher, group, key string, defaultVal int) int {
	val, _ := watcher.GetConfig(group, key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return defaultVal
	}
	return n
}

// ParseIntWithDefault 把字符串转 int，失败或 <=0 时返回默认值。
//
// 适用于非 ConfigWatcher 场景（如 HTTP query 参数）。
// 是 GetIntWithDefault 的无 watcher 版本。
func ParseIntWithDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return defaultVal
	}
	return n
}
