package sentry

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"

	"NetyAdmin/internal/config"
)

// Init 初始化 Sentry SDK。
// 当 cfg.Sentry.DSN 为空时静默跳过，不影响正常启动。
func Init(cfg config.SentryConfig) error {
	if cfg.DSN == "" {
		slog.Info("Sentry DSN 未配置，跳过初始化（后端错误追踪已禁用）")
		return nil
	}

	// 设置默认值
	env := cfg.Environment
	if env == "" {
		env = "development"
	}
	// SampleRate 默认值处理（M1 修复）：
	//   cfg.SampleRate == nil → TOML 中未配置 sample_rate，使用默认 1.0（全量上报）
	//   cfg.SampleRate != nil → 按配置值（含显式 0=关闭错误上报，这是 Sentry SDK 合法语义）
	// 旧代码用 float64 零值判断"未配置"，导致 config.toml 写 sample_rate=0 想关闭上报时
	// 被强制改为 100% 全量上报；改用 *float64 指针区分"未配置"与"显式 0"。
	var sampleRate float64 = 1.0
	if cfg.SampleRate != nil {
		sampleRate = *cfg.SampleRate
	}

	opts := sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      env,
		Release:          cfg.Release,
		SampleRate:       sampleRate,
		EnableTracing:    cfg.TracesSampleRate > 0,
		TracesSampleRate: cfg.TracesSampleRate,
		// 自动在 capture message 时附带堆栈
		AttachStacktrace: true,
		// 忽略常见非操作类错误
		IgnoreErrors: []string{
			"context deadline exceeded",
			"connection refused",
			"connection reset by peer",
			"i/o timeout",
		},
		// 过滤性能追踪事务（regex 匹配事务名，格式 "<METHOD> <FullPath>"）。
		// sentrygin 默认对每个请求都创建事务，K8s 健康探针/静态资源等高频低价值请求
		// 会污染 Sentry 性能面板（如 /health 每 10s 一次 × 20% 采样 ≈ 1700+/天）。
		IgnoreTransactions: buildIgnoreTransactions(cfg.IgnoreTransactions),
		// BeforeSend: 事件发送前做 PII 脱敏，防止 password/secret/token 等敏感字段值
		// 与用户邮箱/IP 进入 Sentry 平台。详见 scrubEvent。
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			return scrubEvent(event)
		},
	}

	if err := sentry.Init(opts); err != nil {
		return fmt.Errorf("Sentry 初始化失败: %w", err)
	}

	slog.Info("Sentry 初始化成功",
		"environment", env,
		"release", cfg.Release,
		"sample_rate", sampleRate,
		"traces_sample_rate", cfg.TracesSampleRate,
		"ignore_transactions", len(opts.IgnoreTransactions),
	)
	return nil
}

// defaultIgnoreTransactions 是默认过滤的性能事务名（regex）。
//
// sentrygin 的事务名格式固定为 "<METHOD> <FullPath>"（如 "GET /health"），
// sentry-go 的 IgnoreTransactions 匹配逻辑是「regex.Match || strings.Contains」，
// 即子串匹配。为避免裸子串（如 "/assets/"）误伤未来包含该子串的业务路由
// （例如 /admin/v1/assets-management），这里在 path 前加一个空格锚定 method/path 分隔符，
// 确保只匹配 path 起始部分，不匹配 method 名也不匹配 path 中间子串。
//
// 默认覆盖：
//   - " /health"      K8s liveness/readiness 探针（每 10s 一次，最高频噪声源）
//   - " /favicon"     浏览器自动请求的站点图标（/favicon.svg 等）
//   - " /assets/"     前端 SPA 静态资源（JS/CSS/图片）
//
// 用户在 config.toml 配置的 ignore_transactions 会**追加**到默认清单之上（不覆盖），
// 避免用户配置时漏掉探针/静态资源导致噪声再次涌入。
func defaultIgnoreTransactions() []string {
	return []string{
		` /health`,
		` /favicon`,
		` /assets/`,
	}
}

// buildIgnoreTransactions 合并默认清单与用户配置清单，去重。
// 容量预分配按「默认清单长度 + 用户清单长度」精确计算，避免扩容。
func buildIgnoreTransactions(user []string) []string {
	defaults := defaultIgnoreTransactions()
	seen := make(map[string]struct{}, len(defaults)+len(user))
	result := make([]string, 0, len(defaults)+len(user))
	for _, pattern := range defaults {
		if pattern == "" {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		result = append(result, pattern)
	}
	for _, pattern := range user {
		if pattern == "" {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		result = append(result, pattern)
	}
	return result
}

// Flush 刷新 Sentry 缓冲区，确保未发送的事件被提交。
// 应在应用退出前调用（defer sentry.Flush(2 * time.Second)）。
func Flush(timeout time.Duration) {
	sentry.Flush(timeout)
}

// CaptureException 捕获错误并发送到 Sentry。
// 如果 Sentry 未初始化或 DSN 为空，此操作为空操作。
func CaptureException(err error) {
	sentry.CaptureException(err)
}

// CaptureMessage 捕获消息并发送到 Sentry。
func CaptureMessage(msg string) {
	sentry.CaptureMessage(msg)
}

// SetUser 设置当前用户上下文（用于请求级关联）。
//
// PII 审计（SubTask 6.3）：
//   - 此函数仅接收 admin 端 uint userID + username，不接收手机号/邮箱。
//   - 全项目唯一的 sentry.User 注入点为 middleware/recovery.go 的 applySentryUser，
//     其 client 端 userID 取自 c.Get("userID") (= JWT claims.UID = 用户实体 ULID，
//     size:26 字符串)，admin 端取自 c.Get("adminID") (uint)。两者均不含 PII。
//   - 即：User.ID 全链路无手机号/邮箱流入，BeforeSend 的 User.Email/IPAddress 兜底
//     脱敏覆盖了第三方 SDK 自动采集的 IP（sentrygin 会注入 c.ClientIP()）。
func SetUser(userID uint, username string) {
	sentry.CurrentHub().Scope().SetUser(sentry.User{
		ID:       strconv.FormatUint(uint64(userID), 10),
		Username: username,
	})
}

// SetTag 设置全局标签。
func SetTag(key, value string) {
	sentry.CurrentHub().Scope().SetTag(key, value)
}

// --- BeforeSend PII 脱敏 ---
//
// Sentry BeforeSend 钩子在事件发送到 Sentry 平台前调用，是脱敏 PII 的最后一道关口。
// 此处递归 scrub event.User / event.Contexts / event.Request.Data，将命中敏感字段名
// 的值替换为 [REDACTED]，同时直接清空 event.User.Email / event.User.IPAddress。
//
// 注意：sentry-go v0.47.0 的 Event 已移除 Extra 字段，自定义附加数据通过
// scope.SetContext(...) 写入 event.Contexts（map[string]Context，Context = map[string]any），
// 故此处遍历 Contexts 而非 Extra。

// redactedPlaceholder 是 PII 字段值的统一占位符。
const redactedPlaceholder = "[REDACTED]"

// maxScrubDepth 限制递归深度，防止恶意构造的深嵌套 map 导致栈溢出或循环引用。
const maxScrubDepth = 10

// sensitiveKeyPattern 匹配敏感字段名（小写，子串匹配）。
// 命中以下任一关键词的字段值会被替换为 [REDACTED]：
//   password / secret / token / appsecret / app_key / access_key / refresh_token
//
// 采用 unanchored 子串匹配，故 user_password、app_secret、access_token、authTokenV2
// 等变体均能命中；case-insensitive 由调用方 strings.ToLower(key) 保证。
var sensitiveKeyPattern = regexp.MustCompile(`password|secret|token|appsecret|app_key|access_key|refresh_token`)

// isSensitiveKey 判断字段名是否敏感（case-insensitive 子串匹配）。
func isSensitiveKey(key string) bool {
	return sensitiveKeyPattern.MatchString(strings.ToLower(key))
}

// scrubEvent 对 Sentry 事件做 PII 脱敏，返回脱敏后的事件（原地修改）。
// 处理范围：
//   - event.User.Email / event.User.IPAddress：直接替换为 [REDACTED]
//   - event.User.Data（map[string]string）：命中敏感 key 的 value 替换为 [REDACTED]
//   - event.Contexts（map[string]map[string]any）：递归 scrub 每个 context 的键值
//   - event.Request.Data（JSON 字符串）：解析后递归 scrub，再序列化回字符串
//
// event 为 nil 时返回 nil（防御性，避免 BeforeSend panic）。
func scrubEvent(event *sentry.Event) *sentry.Event {
	if event == nil {
		return nil
	}
	// User.Email / User.IPAddress 直接脱敏（sentrygin 会自动注入 c.ClientIP()）
	if event.User.Email != "" {
		event.User.Email = redactedPlaceholder
	}
	if event.User.IPAddress != "" {
		event.User.IPAddress = redactedPlaceholder
	}
	// User.Data：map[string]string，按 key 脱敏 value
	for k := range event.User.Data {
		if isSensitiveKey(k) {
			event.User.Data[k] = redactedPlaceholder
		}
	}
	// Contexts：递归脱敏每个 context 的 map[string]any
	for _, ctx := range event.Contexts {
		scrubMap(ctx, 0)
	}
	// Request.Data：JSON 字符串解析后递归脱敏
	if event.Request != nil && event.Request.Data != "" {
		event.Request.Data = scrubJSONString(event.Request.Data)
	}
	return event
}

// scrubMap 递归 scrub map[string]any，命中敏感 key 的值替换为 [REDACTED]，
// 其余值递归处理。depth 超过 maxScrubDepth 时停止递归（保留原值，防循环引用）。
func scrubMap(m map[string]any, depth int) {
	if depth > maxScrubDepth {
		return
	}
	for k, v := range m {
		if isSensitiveKey(k) {
			m[k] = redactedPlaceholder
			continue
		}
		m[k] = scrubValue(v, depth+1)
	}
}

// scrubValue 递归 scrub 任意值，处理 map[string]any 与 []any。
func scrubValue(v any, depth int) any {
	if depth > maxScrubDepth {
		return v
	}
	switch val := v.(type) {
	case map[string]any:
		scrubMap(val, depth)
		return val
	case []any:
		for i, item := range val {
			val[i] = scrubValue(item, depth+1)
		}
		return val
	default:
		return v
	}
}

// scrubJSONString 解析 JSON 字符串，递归 scrub 后重新序列化。
// 解析失败（非合法 JSON）时原样返回，避免破坏事件可读性。
func scrubJSONString(data string) string {
	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return data
	}
	scrubbed := scrubValue(parsed, 0)
	out, err := json.Marshal(scrubbed)
	if err != nil {
		return data
	}
	return string(out)
}
