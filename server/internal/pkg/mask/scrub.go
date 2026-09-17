package mask

import "regexp"

// sensitiveSubstringPattern 敏感字段名的宽松子串匹配（大小写不敏感）。
// 关键词并集覆盖两处历史清单：开放平台 HTTP 头/体脱敏与 Sentry PII 脱敏，
// 收敛到本包单一事实源（原先三份各自维护，关键词集与占位符互不一致）。
var sensitiveSubstringPattern = regexp.MustCompile(`(?i)(password|passwd|pwd|secret|signature|token|api_?key|app_?key|access_?key|private_?key|credit_?card|cvv|ssn)`)

// IsSensitiveSubstr 宽松匹配：字段名包含任一敏感关键词子串（含 api_key/appKey 等分隔形态）。
// 与 IsSensitive（归一化精确匹配）互补：日志键值脱敏用本函数，操作日志按键删除用 IsSensitive。
func IsSensitiveSubstr(key string) bool {
	return sensitiveSubstringPattern.MatchString(key)
}

// ScrubValue 递归脱敏任意 JSON 形值：命中 IsSensitiveSubstr 的 key 其值替换为 placeholder，
// map/slice 递归处理，其余原样返回。maxDepth 限制递归深度（防超深嵌套拖垮调用方，超深保留原值）。
//
// 全项目唯一的脱敏遍历器（原 middleware/open_platform_auth 与 pkg/sentry 各持一份）。
func ScrubValue(v interface{}, placeholder string, maxDepth int) interface{} {
	return scrubValue(v, placeholder, 0, maxDepth)
}

func scrubValue(v interface{}, placeholder string, depth, maxDepth int) interface{} {
	if depth > maxDepth {
		return v
	}
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, vv := range val {
			if IsSensitiveSubstr(k) {
				result[k] = placeholder
			} else {
				result[k] = scrubValue(vv, placeholder, depth+1, maxDepth)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, vv := range val {
			result[i] = scrubValue(vv, placeholder, depth+1, maxDepth)
		}
		return result
	default:
		return v
	}
}
