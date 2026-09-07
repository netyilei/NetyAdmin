package storage

import (
	"errors"
	"fmt"
	"strings"
)

// BuildPublicURL 构造对象的公开访问 URL，是全项目唯一真相源
// （重构清单 B-OTHER-1：消除 minio_driver.buildURL / record.go domain 解析 / config.go endpoint 回退 三处不一致）。
//
// 接受离散参数而非整个 Config struct，因为项目存在两个 Config 类型：
//   - pkg/storage.Config（驱动配置）
//   - domain/entity/storage.Config（DB 实体）
//
// 两者字段名一致但类型不同，离散参数避免类型耦合。
//
// 优先级：
//  1. domain 非空 → 用 domain（规范化协议+只保留 host，再拼 key）
//  2. domain 空 → 按 style 与 endpoint 实际协议拼桶地址：
//     - StyleVirtualHost: {scheme}://{bucket}.{endpoint-host}/{key}
//     - StylePath:        {scheme}://{endpoint-host}/{bucket}/{key}
//
// 兼容 domain 多种写法：
//   - "https://cdn.example.com"          → https://cdn.example.com/{key}
//   - "https://cdn.example.com/sub/path" → https://cdn.example.com/{key}（剥掉子路径，保持 host-only）
//   - "cdn.example.com"                  → https://cdn.example.com/{key}（无协议时默认 https://）
func BuildPublicURL(domain, endpoint, bucket, key string, style AddressingStyle) string {
	key = strings.TrimPrefix(key, "/")

	// 1. 自定义域名优先
	if domain != "" {
		return joinDomainKey(normalizeDomain(domain), key)
	}

	// 2. 回退到 endpoint。调用方传入的 endpoint 均已通过驱动构造时的
	// ParseEndpoint 校验，此处错误仅作理论防御（按 http + 裸字符串降级拼接）。
	host, secure, err := ParseEndpoint(endpoint)
	if err != nil {
		host, secure = strings.TrimSuffix(stripProtocol(endpoint), "/"), false
	}
	scheme := "http"
	if secure {
		scheme = "https"
	}

	if style == StylePath {
		return scheme + "://" + host + "/" + bucket + "/" + key
	}
	return scheme + "://" + bucket + "." + host + "/" + key
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

// ParseEndpoint 将 endpoint 配置统一解析为裸地址 + 协议（全项目唯一解析点，
// 驱动构造与 service 层入口校验共用，保证两层判定口径一致）。
//
// 接受三种形态：
//   - "https://host[:port]" → (host, true)
//   - "http://host[:port]"  → (host, false)
//   - "host[:port]" 裸 host（历史遗留形态）→ (host, false)
//
// 容忍首尾空白与尾部斜杠；内嵌路径（如 https://host/sub）返回明确错误，
// 而不是把含协议/带尾斜杠的字符串透传给 minio-go，由其报出模糊的
// "Endpoint url cannot have fully qualified paths"。
//
// 裸 host 默认 http 是刻意保留的历史语义：存量本地 MinIO 配置以裸 host
// 形态工作，默认 https 会在升级瞬间将其打死；新配置由服务层入口校验
// 强制携带协议前缀。
func ParseEndpoint(raw string) (host string, secure bool, err error) {
	e := strings.TrimSpace(raw)

	switch {
	case strings.HasPrefix(e, "https://"):
		secure = true
		e = strings.TrimPrefix(e, "https://")
	case strings.HasPrefix(e, "http://"):
		e = strings.TrimPrefix(e, "http://")
	}

	e = strings.TrimSuffix(e, "/")

	if e == "" {
		return "", false, errors.New("endpoint 不能为空")
	}
	if strings.Contains(e, "/") {
		return "", false, fmt.Errorf("endpoint 不支持携带路径: %s", raw)
	}

	return e, secure, nil
}
