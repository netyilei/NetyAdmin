package storage

import (
	"strings"
)

// BuildPublicURL 构造对象的公开访问 URL，是全项目唯一真相源
// （重构清单 B-OTHER-1：消除 minio_driver.buildURL / record.go domain 解析 / config.go endpoint 回退 三处不一致）。
//
// 接受离散参数而非整个 Config struct，因为项目存在两个 Config 类型：
//   - pkg/storage.Config（驱动配置）
//   - domain/entity/storage.Config（DB 实体）
// 两者字段名一致但类型不同，离散参数避免类型耦合。
//
// 优先级：
//  1. domain 非空 → 用 domain（规范化协议+只保留 host，再拼 key）
//  2. domain 空 → 按 endpoint 拼桶名虚拟主机风格：https://{bucket}.{endpoint-host}/{key}
//
// 兼容 domain 多种写法：
//   - "https://cdn.example.com"          → https://cdn.example.com/{key}
//   - "https://cdn.example.com/sub/path" → https://cdn.example.com/{key}（剥掉子路径，保持 host-only）
//   - "cdn.example.com"                  → https://cdn.example.com/{key}（无协议时默认 https://）
func BuildPublicURL(domain, endpoint, bucket, key string) string {
	key = strings.TrimPrefix(key, "/")

	// 1. 自定义域名优先
	if domain != "" {
		return joinDomainKey(normalizeDomain(domain), key)
	}

	// 2. 回退到 endpoint 虚拟主机风格
	host := stripProtocol(endpoint)
	return "https://" + bucket + "." + host + "/" + key
}

// normalizeDomain 规范化 domain：
//   - 补默认协议 https://（若缺失）
//   - 剥掉 path 部分，只保留 scheme://host
//
// 例：
//
//	"cdn.example.com"             → "https://cdn.example.com"
//	"http://cdn.example.com/x/y"  → "http://cdn.example.com"
//	"https://oss.example.com"     → "https://oss.example.com"
func normalizeDomain(domain string) string {
	scheme := "https"
	rest := domain
	if strings.HasPrefix(domain, "https://") {
		rest = strings.TrimPrefix(domain, "https://")
	} else if strings.HasPrefix(domain, "http://") {
		scheme = "http"
		rest = strings.TrimPrefix(domain, "http://")
	}
	// 只保留 host，剥掉 path
	if idx := strings.Index(rest, "/"); idx > 0 {
		rest = rest[:idx]
	}
	return scheme + "://" + rest
}

// joinDomainKey 拼接 domain + key，避免双斜杠。
func joinDomainKey(domain, key string) string {
	return strings.TrimSuffix(domain, "/") + "/" + strings.TrimPrefix(key, "/")
}

// stripProtocol 剥掉 endpoint 的协议前缀，返回 host[:port]。
func stripProtocol(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return endpoint
}
