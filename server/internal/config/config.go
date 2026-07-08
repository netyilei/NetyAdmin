package config

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server          ServerConfig          `toml:"server"`
	TLS             TLSConfig             `toml:"tls"`
	Database        DatabaseConfig        `toml:"database"`
	Redis           RedisConfig           `toml:"redis"`
	JWT             JWTConfig             `toml:"jwt"`
	Log             LogConfig             `toml:"log"`
	Migration       MigrationConfig       `toml:"migration"`
	Task            TaskConfig            `toml:"task"`
	Security        SecurityConfig        `toml:"security"`
	Email           EmailConfig           `toml:"email"`
	Sms             SmsConfig             `toml:"sms"`
	Bus             BusConfig             `toml:"bus"`
	PubSub          PubSubConfig          `toml:"pubsub"`
	Sentry          SentryConfig          `toml:"sentry"`
	CORS            CORSConfig            `toml:"cors"`
	SecurityHeaders SecurityHeadersConfig `toml:"security_headers"`
	LoginRateLimit  LoginRateLimitConfig  `toml:"login_ratelimit"`
}

// Duration 是 time.Duration 的包装类型，实现 encoding.TextUnmarshaler 接口，
// 使 go-toml/v2 能将 TOML 字符串（如 "30m"/"168h"/"25s"）解析为 time.Duration。
//
// 背景：go-toml/v2 原生不支持 time.Duration——它是 int64 的命名类型别名，
// 不实现 TextUnmarshaler，直接 toml.Unmarshal 会报错：
// "cannot decode TOML string into struct field of type time.Duration"。
// 通过包装类型 + UnmarshalText 让 TOML 字符串走 time.ParseDuration 解析路径。
//
// 业务代码使用时需调用 .Duration() 方法转回 time.Duration（用于 jwt.New / http.Server 等）。
type Duration time.Duration

// UnmarshalText 实现 encoding.TextUnmarshaler，支持 "30m"/"168h"/"25s" 等 time.ParseDuration 格式。
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", string(text), err)
	}
	*d = Duration(parsed)
	return nil
}

// Duration 返回底层 time.Duration 值，供业务代码使用。
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// TLSConfig HTTPS 配置。
// 默认关闭：默认部署架构由前端 Nginx 终止 TLS，后端只听 HTTP。
// 若后端直接对外暴露（无 Nginx/CDN），应启用 HTTPS 并配置 cert_file / key_file。
type TLSConfig struct {
	Enable   bool   `toml:"enable"`
	CertFile string `toml:"cert_file"`
	KeyFile  string `toml:"key_file"`
}

// CORSConfig 跨域资源共享配置。
// AllowedOrigins 为允许的来源白名单（精确匹配，不支持通配符）。
// 空列表 = 拒绝所有跨域请求（fail-closed），生产环境必须显式配置可信来源。
type CORSConfig struct {
	AllowedOrigins []string `toml:"allowed_origins" env:"NETYADMIN_CORS_ALLOWED_ORIGINS"` // 允许的来源白名单；环境变量用逗号分隔
}

// SecurityHeadersConfig 安全响应头配置。
// CSP 为空时不设置 Content-Security-Policy 头（保持兼容性，不强制）。
type SecurityHeadersConfig struct {
	CSP string `toml:"csp" env:"NETYADMIN_SECURITY_HEADERS_CSP"` // Content-Security-Policy 头内容
}

// LoginRateLimitConfig 登录端点 IP 维度限流配置。
// 仅作用于 admin /auth/login + /auth/refresh-token 与 client /user/login + /user/refresh-token 路由，
// 不影响其他接口。算法为 Redis ZSET 滑动窗口（ZADD + ZREMRANGEBYSCORE + ZCARD）。
// Redis 未配置（cache disabled）时限流器降级为 no-op（fail-open），不阻断登录关键路径。
type LoginRateLimitConfig struct {
	// Window 滑动窗口时长（如 "1m" / "5m"）。零值 = 默认 1m。
	// 环境变量：NETYADMIN_LOGIN_RATELIMIT_WINDOW
	// 类型为 Duration（实现 encoding.TextUnmarshaler），使 go-toml/v2 能解析 "1m" 等字符串。
	Window Duration `toml:"window" env:"NETYADMIN_LOGIN_RATELIMIT_WINDOW"`
	// Max 窗口内单个 IP 允许的最大登录尝试次数。零值 = 默认 10。
	// 环境变量：NETYADMIN_LOGIN_RATELIMIT_MAX
	Max int `toml:"max" env:"NETYADMIN_LOGIN_RATELIMIT_MAX"`
}

// SentryConfig Sentry 错误追踪配置（后端 Go）
// DSN 为空时自动禁用，不影响正常运行
type SentryConfig struct {
	DSN                string   `toml:"dsn" env:"NETYADMIN_SENTRY_DSN"`                 // Sentry DSN，为空则禁用
	Environment        string   `toml:"environment" env:"NETYADMIN_SENTRY_ENVIRONMENT"` // 环境标识（development / production）
	Release            string   `toml:"release" env:"NETYADMIN_SENTRY_RELEASE"`         // 版本号
	SampleRate         *float64 `toml:"sample_rate"`                                    // 错误事件采样率 (0.0-1.0)；nil=未配置默认1.0，显式0=关闭错误上报
	TracesSampleRate   float64  `toml:"traces_sample_rate"`                             // 性能追踪采样率 (0.0-1.0)；0=关闭性能追踪
	IgnoreTransactions []string `toml:"ignore_transactions"`                            // 需过滤的性能事务名（regex），默认含 /health 等探针/静态资源噪声
}

type EmailConfig struct {
	Enabled        bool   `toml:"enabled"`
	Host           string `toml:"host"`
	Port           int    `toml:"port"`
	User           string `toml:"user"`
	Password       string `toml:"password" env:"NETYADMIN_EMAIL_PASSWORD"`
	From           string `toml:"from"`
	SSL            bool   `toml:"ssl"`
	StartTLS       bool   `toml:"starttls"`
	AuthType       string `toml:"auth_type"`
	ConnectTimeout int    `toml:"connect_timeout"`
	SendTimeout    int    `toml:"send_timeout"`
}

type SmsConfig struct {
	Enabled   bool   `toml:"enabled"`
	Driver    string `toml:"driver"`
	SecretID  string `toml:"secret_id" env:"NETYADMIN_SMS_SECRET_ID"`
	SecretKey string `toml:"secret_key" env:"NETYADMIN_SMS_SECRET_KEY"`
	AppID     string `toml:"app_id"`
	SignName  string `toml:"sign_name"`
	Region    string `toml:"region" env:"NETYADMIN_SMS_REGION"`
}

type SecurityConfig struct {
	AESKey        string `toml:"aes_key" env:"NETYADMIN_AES_KEY"` // 系统加解密 Key (16, 24 或 32 字节)
	UploadHMACKey string `toml:"upload_hmac_key" env:"NETYADMIN_UPLOAD_HMAC_KEY"`
}

type TaskConfig struct {
	Enabled bool                 `toml:"enabled"`
	Workers int                  `toml:"workers"`
	Jobs    map[string]JobConfig `toml:"jobs"`
}

type JobConfig struct {
	Enabled *bool   `toml:"enabled"` // 是否启用
	Type    *string `toml:"type"`    // 模式覆盖: once, cron, interval
	Spec    *string `toml:"spec"`    // 参数覆盖: 间隔时间或 Cron 表达式
	Weight  *int    `toml:"weight"`  // 权重覆盖 (0-100)
}

// MigrationConfig 数据库迁移配置。
// Dir 字段已移除：迁移文件以 go:embed 编译进二进制，不再从外部目录读取。
type MigrationConfig struct {
	Enabled bool `toml:"enabled"`
}

type ServerConfig struct {
	Port         int    `toml:"port" env:"NETYADMIN_SERVER_PORT"`
	Mode         string `toml:"mode" env:"NETYADMIN_SERVER_MODE"`
	ReadTimeout  int    `toml:"read_timeout"`
	WriteTimeout int    `toml:"write_timeout"`
	// HandlerTimeout 是 http.TimeoutHandler 的请求处理超时阈值。
	// 应略小于 ReadTimeout/WriteTimeout，确保超时时由中间件返回 503 + JSON 错误体，
	// 而非连接层超时断开（客户端会收到空响应 / 连接重置）。
	// 零值 = 默认 25s（在 app.go 中兜底）。
	// 类型为 Duration（实现 encoding.TextUnmarshaler），使 go-toml/v2 能解析 "25s" 等字符串。
	HandlerTimeout Duration `toml:"handler_timeout" env:"NETYADMIN_SERVER_HANDLER_TIMEOUT"`
	// ShutdownTimeout 是优雅关闭时 srv.Shutdown 等待在途请求的最大时长。
	// 零值 = 默认 30s（在 app.go 中兜底）。
	// 类型为 Duration（实现 encoding.TextUnmarshaler），使 go-toml/v2 能解析 "30s" 等字符串。
	ShutdownTimeout Duration `toml:"shutdown_timeout" env:"NETYADMIN_SERVER_SHUTDOWN_TIMEOUT"`
	MultiNode       bool          `toml:"multi_node"` // 多机部署设为 true：会校验事件总线是否为 redis 模式
	// TrustedProxies 可信代理 IP 或 IPv4 CIDR 列表（如 ["127.0.0.1", "10.0.0.0/8"]）。
	// 空数组（默认）= 不信任任何代理：c.ClientIP() 直接回退到 RemoteAddr，忽略 X-Forwarded-For / X-Real-IP 头，
	// 防止攻击者伪造 IP 头绕过 IPAC 白名单/黑名单。
	// 生产环境若部署在 Nginx/CDN 之后，必须填写真实代理 IP/CIDR 才能正确解析客户端真实 IP。
	TrustedProxies []string `toml:"trusted_proxies"`
}

type DatabaseConfig struct {
	Host     string `toml:"host" env:"NETYADMIN_DB_HOST"`
	Port     int    `toml:"port" env:"NETYADMIN_DB_PORT"`
	User     string `toml:"user" env:"NETYADMIN_DB_USER"`
	Password string `toml:"password" env:"NETYADMIN_DB_PASSWORD"`
	DBName   string `toml:"dbname" env:"NETYADMIN_DB_NAME"`
	SSLMode  string `toml:"sslmode" env:"NETYADMIN_DB_SSLMODE"`
	MaxIdle  int    `toml:"max_idle"`
	MaxOpen  int    `toml:"max_open"`
}

// DSN 返回 PostgreSQL 的 keyword=value 格式 DSN（pgx 通用）。
// 用于 GORM 连接（GORM postgres driver 使用 pgx 底层，接受 keyword=value 格式）。
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// DSNURL 返回 PostgreSQL 的 URL 格式 DSN（golang-migrate / lib/pq 通用）。
// golang-migrate 的 postgres driver 需要 URL 格式（带 postgres:// 协议头）。
func (c DatabaseConfig) DSNURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode)
}

type RedisConfig struct {
	Enabled  bool   `toml:"enabled"`
	Prefix   string `toml:"prefix"`
	Host     string `toml:"host" env:"NETYADMIN_REDIS_HOST"`
	Port     int    `toml:"port" env:"NETYADMIN_REDIS_PORT"`
	Password string `toml:"password" env:"NETYADMIN_REDIS_PASSWORD"`
	DB       int    `toml:"db"`

	PoolSize       int `toml:"pool_size" env:"NETYADMIN_REDIS_POOL_SIZE"`
	MinIdleConns   int `toml:"min_idle_conns" env:"NETYADMIN_REDIS_MIN_IDLE_CONNS"`
	PoolTimeoutSec int `toml:"pool_timeout_sec" env:"NETYADMIN_REDIS_POOL_TIMEOUT_SEC"`

	L1Enabled       bool `toml:"l1_enabled"`
	LocalMaxSizeMB  int  `toml:"local_max_size_mb"`
	LocalMaxEntryKB int  `toml:"local_max_entry_kb"`
	LocalTTLMin     int  `toml:"local_ttl_min"`
}

type BusConfig struct {
	Driver string `toml:"driver" env:"NETYADMIN_BUS_DRIVER"` // "redis" | "memory"，默认根据 Redis.Enabled 自动选择
}

// PubSubConfig 事件总线分发工作池配置（Task 23）。
// dispatch 阶段（消费 loop → handler）使用 buffered channel + N workers 模式，
// 替代原本的 per-event goroutine，避免高吞吐场景下 goroutine 数量爆炸。
//
// 行为：
//   - Workers：消费 dispatchQueue 的 worker 协程数；零值 = 默认 16（在 pubsub.NewMemoryDriver/NewRedisDriver 中兜底）。
//   - QueueSize：dispatchQueue 的缓冲容量；零值 = 默认 1024。
//   - 队列满时 dispatch 阻塞（backpressure），让消费 loop 反压到上游 Publish。
//   - 关闭时（Close）先停止消费 loop，再关闭 dispatchQueue，worker 排空后退出。
type PubSubConfig struct {
	Workers   int `toml:"workers" env:"NETYADMIN_PUBSUB_WORKERS"`       // worker 协程数，零值 = 默认 16
	QueueSize int `toml:"queue_size" env:"NETYADMIN_PUBSUB_QUEUE_SIZE"` // dispatch 队列容量，零值 = 默认 1024
}

// JWTConfig RS256 非对称签名 + Access/Refresh TTL 独立配置。
//   - 私钥/公钥均支持 file path 或内联 PEM 两种加载方式，file path 优先。
//   - 生产模式 fail-closed：私钥/公钥必须配置（file 或 PEM 至少一项），否则启动失败。
//   - AccessTokenTTL 短时（默认 30m），RefreshTokenTTL 长时（默认 168h = 7 天）。
//   - TTL 字段类型为 Duration（实现 encoding.TextUnmarshaler），使 go-toml/v2 能解析 "30m" 等字符串。
type JWTConfig struct {
	PrivateKeyFile  string   `toml:"private_key_file"`
	PrivateKeyPEM   string   `toml:"private_key_pem"`
	PublicKeyFile   string   `toml:"public_key_file"`
	PublicKeyPEM    string   `toml:"public_key_pem"`
	AccessTokenTTL  Duration `toml:"access_token_ttl"`
	RefreshTokenTTL Duration `toml:"refresh_token_ttl"`
}

type LogConfig struct {
	Level      string `toml:"level"`
	Filename   string `toml:"filename"`
	MaxSize    int    `toml:"max_size"`
	MaxBackups int    `toml:"max_backups"`
	MaxAge     int    `toml:"max_age"`
	Compress   bool   `toml:"compress"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 12-factor: 环境变量优先于 TOML，覆盖带 env 标签的字段
	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("应用环境变量覆盖失败: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides 通过 reflect 遍历 Config 结构体，对带 `env:"NETYADMIN_XXX"` 标签的叶子字段
// 用环境变量值覆盖 TOML 已解析的值。环境变量未设置时保留 TOML 值；显式设置为空字符串时覆盖为空。
// 优先级：环境变量 > TOML > 零值。
func applyEnvOverrides(c *Config) error {
	v := reflect.ValueOf(c).Elem()
	return walkFields(v)
}

// walkFields 递归遍历结构体字段：命中 env 标签的叶子字段执行环境变量覆盖；
// 否则继续向下递归嵌套 struct（map / slice 字段不递归）。
func walkFields(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		fv := v.Field(i)
		// 命中 env 标签：尝试环境变量覆盖
		if envKey := field.Tag.Get("env"); envKey != "" {
			if val, ok := os.LookupEnv(envKey); ok {
				if err := setFieldFromString(fv, val); err != nil {
					return fmt.Errorf("env %s=%q 解析失败: %w", envKey, val, err)
				}
			}
			continue
		}
		// 递归嵌套 struct（map / slice 不递归，env 标签仅在叶子字段生效）
		if fv.Kind() == reflect.Struct {
			if err := walkFields(fv); err != nil {
				return err
			}
		}
	}
	return nil
}

// setFieldFromString 将环境变量字符串写入 reflect.Value，支持 string / int / bool / time.Duration / Duration。
// 不支持的类型返回错误，避免静默忽略覆盖。
func setFieldFromString(fv reflect.Value, val string) error {
	if !fv.CanSet() {
		return fmt.Errorf("字段不可设置")
	}
	// time.Duration 与本项目 Duration 包装类型都是基于 int64 的命名类型，
	// 需用 time.ParseDuration 解析（如 "30s" / "5m"），
	// 否则会被当作裸 int64 纳秒处理，env 体验极差。
	// Duration 与 time.Duration 底层均为 int64，统一用 SetInt 写入即可。
	if fv.Type() == reflect.TypeOf(time.Duration(0)) || fv.Type() == reflect.TypeOf(Duration(0)) {
		d, err := time.ParseDuration(val)
		if err != nil {
			return err
		}
		fv.SetInt(int64(d))
		return nil
	}
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	case reflect.Slice:
		// 仅支持 []string（逗号分隔）；空字符串 = 空切片
		if fv.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("暂不支持切片元素类型 %s 的环境变量覆盖", fv.Type().Elem().Kind())
		}
		parts := splitCSV(val)
		slice := reflect.MakeSlice(fv.Type(), len(parts), len(parts))
		for i, p := range parts {
			slice.Index(i).SetString(p)
		}
		fv.Set(slice)
	default:
		return fmt.Errorf("暂不支持类型 %s 的环境变量覆盖", fv.Kind())
	}
	return nil
}

// splitCSV 将逗号分隔的字符串拆分为切片，每个元素去除首尾空白。
// 空字符串返回 nil 切片（与"未配置"语义等价：fail-closed）。
func splitCSV(val string) []string {
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		// 不跳过空段：保留显式空值以便表达"含空 origin"，但通常无意义
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// ValidateConfig 在非 debug 模式下校验敏感配置不得为默认值或占位符。
// 任一校验失败则 log.Fatal 拒绝启动，避免敏感配置泄漏到生产环境。
// 应在 main.go 中 config.Load 之后、InitDB 之前调用。
//
// 校验项：
//   - [database].password         不得为 "123456" / "<CHANGE_ME_IN_PRODUCTION>"
//   - [jwt].private_key_*         私钥必须配置（file 或 PEM 至少一项）
//   - [jwt].public_key_*          公钥必须配置（file 或 PEM 至少一项）
//   - [jwt].access_token_ttl      必须 > 0
//   - [jwt].refresh_token_ttl     必须 > 0
//   - [security].aes_key          不得为 "netyadmin-aes-key-32-chars-long!" / "<CHANGE_ME_IN_PRODUCTION>"
//   - [security].upload_hmac_key  不得为空 / "<CHANGE_ME_IN_PRODUCTION>"（替换原 [jwt].secret 复用）
//   - [email].password            （仅 Email.Enabled）不得为 "your-password" / "<CHANGE_ME_IN_PRODUCTION>"
//   - [sms].secret_id             （仅 Sms.Enabled）不得为空 / "<CHANGE_ME_IN_PRODUCTION>"
//   - [sms].secret_key            （仅 Sms.Enabled）不得为空 / "<CHANGE_ME_IN_PRODUCTION>"
//   - [sms].region                （仅 Sms.Enabled）不得为空（腾讯云接入地域，如 ap-guangzhou）
//   - [sms].app_id                （仅 Sms.Enabled）不得为空（腾讯云 SmsSdkAppId）
//   - [sms].sign_name             （仅 Sms.Enabled）不得为空（短信签名）
//   - [redis].password            （仅 Redis.Enabled）不得为空 / "<CHANGE_ME_IN_PRODUCTION>"
func ValidateConfig(cfg *Config) {
	if cfg == nil {
		log.Fatal("配置校验失败: cfg 为 nil")
	}
	if cfg.Server.Mode == "debug" {
		return
	}

	// 不允许的默认/占位符值集合
	forbiddenDBPwd := map[string]struct{}{
		"123456":                    {},
		"<CHANGE_ME_IN_PRODUCTION>": {},
	}
	forbiddenAES := map[string]struct{}{
		"netyadmin-aes-key-32-chars-long!": {},
		"<CHANGE_ME_IN_PRODUCTION>":        {},
	}
	forbiddenUploadHMAC := map[string]struct{}{
		"":                          {},
		"<CHANGE_ME_IN_PRODUCTION>": {},
	}
	forbiddenEmailPwd := map[string]struct{}{
		"your-password":             {},
		"<CHANGE_ME_IN_PRODUCTION>": {},
	}
	// SMS SecretID/SecretKey 在 config.example.toml 默认为空字符串，故空值也视为未配置。
	forbiddenSmsSecretID := map[string]struct{}{
		"":                          {},
		"<CHANGE_ME_IN_PRODUCTION>": {},
	}
	forbiddenSmsSecretKey := map[string]struct{}{
		"":                          {},
		"<CHANGE_ME_IN_PRODUCTION>": {},
	}
	// Redis password 在 config.example.toml 默认为空字符串，生产环境启用 Redis 时必须显式配置。
	forbiddenRedisPwd := map[string]struct{}{
		"":                          {},
		"<CHANGE_ME_IN_PRODUCTION>": {},
	}

	if _, bad := forbiddenDBPwd[cfg.Database.Password]; bad {
		log.Fatalf("配置校验失败: [database].password 在生产模式下不得为默认值或占位符，请通过环境变量 NETYADMIN_DB_PASSWORD 设置真实密码")
	}
	// JWT RS256 私钥/公钥 fail-closed：file 和 PEM 均为空则拒绝启动
	if cfg.JWT.PrivateKeyFile == "" && cfg.JWT.PrivateKeyPEM == "" {
		log.Fatalf("配置校验失败: [jwt].private_key_file 或 [jwt].private_key_pem 必须配置 RS256 私钥（生产模式 fail-closed）")
	}
	if cfg.JWT.PublicKeyFile == "" && cfg.JWT.PublicKeyPEM == "" {
		log.Fatalf("配置校验失败: [jwt].public_key_file 或 [jwt].public_key_pem 必须配置 RS256 公钥（生产模式 fail-closed）")
	}
	if cfg.JWT.AccessTokenTTL <= 0 {
		log.Fatalf("配置校验失败: [jwt].access_token_ttl 必须 > 0，当前值 %s", cfg.JWT.AccessTokenTTL.Duration())
	}
	if cfg.JWT.RefreshTokenTTL <= 0 {
		log.Fatalf("配置校验失败: [jwt].refresh_token_ttl 必须 > 0，当前值 %s", cfg.JWT.RefreshTokenTTL.Duration())
	}
	if _, bad := forbiddenAES[cfg.Security.AESKey]; bad {
		log.Fatalf("配置校验失败: [security].aes_key 在生产模式下不得为默认值或占位符，请通过环境变量 NETYADMIN_AES_KEY 设置真实密钥")
	}
	if _, bad := forbiddenUploadHMAC[cfg.Security.UploadHMACKey]; bad {
		log.Fatalf("配置校验失败: [security].upload_hmac_key 在生产模式下不得为空或占位符，请通过环境变量 NETYADMIN_UPLOAD_HMAC_KEY 设置真实密钥")
	}
	if cfg.Email.Enabled {
		if _, bad := forbiddenEmailPwd[cfg.Email.Password]; bad {
			log.Fatalf("配置校验失败: [email].password 在生产模式下不得为默认值或占位符，请通过环境变量 NETYADMIN_EMAIL_PASSWORD 设置真实密码")
		}
	}
	if cfg.Sms.Enabled {
		if _, bad := forbiddenSmsSecretID[cfg.Sms.SecretID]; bad {
			log.Fatalf("配置校验失败: [sms].secret_id 在生产模式下不得为空或占位符，请通过环境变量 NETYADMIN_SMS_SECRET_ID 设置真实密钥")
		}
		if _, bad := forbiddenSmsSecretKey[cfg.Sms.SecretKey]; bad {
			log.Fatalf("配置校验失败: [sms].secret_key 在生产模式下不得为空或占位符，请通过环境变量 NETYADMIN_SMS_SECRET_KEY 设置真实密钥")
		}
		// 腾讯云 SendSms 必传项：region / app_id / sign_name。
		// 缺失时启动期 fail-closed，避免「启动成功但首次发短信才报错」。
		if cfg.Sms.Region == "" {
			log.Fatalf("配置校验失败: [sms].region 不得为空（腾讯云接入地域，如 ap-guangzhou），请通过环境变量 NETYADMIN_SMS_REGION 设置")
		}
		if cfg.Sms.AppID == "" {
			log.Fatalf("配置校验失败: [sms].app_id 不得为空（腾讯云短信 SmsSdkAppId，控制台 → 短信 → 应用管理）")
		}
		if cfg.Sms.SignName == "" {
			log.Fatalf("配置校验失败: [sms].sign_name 不得为空（短信签名，需在腾讯云控制台审核通过）")
		}
	}
	if cfg.Redis.Enabled {
		if _, bad := forbiddenRedisPwd[cfg.Redis.Password]; bad {
			log.Fatalf("配置校验失败: [redis].password 在生产模式下不得为空或占位符，请通过环境变量 NETYADMIN_REDIS_PASSWORD 设置真实密码")
		}
	}
}
