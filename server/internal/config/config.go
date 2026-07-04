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
// 仅作用于 admin /auth/login + /auth/refreshToken 与 client /user/login + /user/refresh-token 路由，
// 不影响其他接口。算法为 Redis ZSET 滑动窗口（ZADD + ZREMRANGEBYSCORE + ZCARD）。
// Redis 未配置（cache disabled）时限流器降级为 no-op（fail-open），不阻断登录关键路径。
type LoginRateLimitConfig struct {
	// Window 滑动窗口时长（如 "1m" / "5m"）。零值 = 默认 1m。
	// 环境变量：NETYADMIN_LOGIN_RATELIMIT_WINDOW
	Window time.Duration `toml:"window" env:"NETYADMIN_LOGIN_RATELIMIT_WINDOW"`
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
}

type SecurityConfig struct {
	AESKey string `toml:"aes_key" env:"NETYADMIN_AES_KEY"` // 系统加解密 Key (16, 24 或 32 字节)
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
	HandlerTimeout time.Duration `toml:"handler_timeout" env:"NETYADMIN_SERVER_HANDLER_TIMEOUT"`
	// ShutdownTimeout 是优雅关闭时 srv.Shutdown 等待在途请求的最大时长。
	// 零值 = 默认 30s（在 app.go 中兜底）。
	ShutdownTimeout time.Duration `toml:"shutdown_timeout" env:"NETYADMIN_SERVER_SHUTDOWN_TIMEOUT"`
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

type JWTConfig struct {
	Secret     string `toml:"secret" env:"NETYADMIN_JWT_SECRET"`
	Expiration int    `toml:"expiration"`
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

// setFieldFromString 将环境变量字符串写入 reflect.Value，支持 string / int / bool / time.Duration。
// 不支持的类型返回错误，避免静默忽略覆盖。
func setFieldFromString(fv reflect.Value, val string) error {
	if !fv.CanSet() {
		return fmt.Errorf("字段不可设置")
	}
	// time.Duration 是基于 int64 的命名类型，需用 time.ParseDuration 解析（如 "30s" / "5m"），
	// 否则会被当作裸 int64 纳秒处理，env 体验极差。
	if fv.Type() == reflect.TypeOf(time.Duration(0)) {
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
//   - [database].password 不得为 "123456" / "<CHANGE_ME_IN_PRODUCTION>"
//   - [jwt].secret        不得为 "your-secret-key-change-in-production" / "<CHANGE_ME_IN_PRODUCTION>"
//   - [security].aes_key  不得为 "netyadmin-aes-key-32-chars-long!" / "<CHANGE_ME_IN_PRODUCTION>"
//   - [email].password    （仅 Email.Enabled）不得为 "your-password" / "<CHANGE_ME_IN_PRODUCTION>"
//   - [sms].secret_id     （仅 Sms.Enabled）不得为空 / "<CHANGE_ME_IN_PRODUCTION>"
//   - [sms].secret_key    （仅 Sms.Enabled）不得为空 / "<CHANGE_ME_IN_PRODUCTION>"
//   - [redis].password    （仅 Redis.Enabled）不得为空 / "<CHANGE_ME_IN_PRODUCTION>"
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
	forbiddenJWT := map[string]struct{}{
		"your-secret-key-change-in-production": {},
		"<CHANGE_ME_IN_PRODUCTION>":            {},
	}
	forbiddenAES := map[string]struct{}{
		"netyadmin-aes-key-32-chars-long!": {},
		"<CHANGE_ME_IN_PRODUCTION>":        {},
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
	if _, bad := forbiddenJWT[cfg.JWT.Secret]; bad {
		log.Fatalf("配置校验失败: [jwt].secret 在生产模式下不得为默认值或占位符，请通过环境变量 NETYADMIN_JWT_SECRET 设置真实密钥")
	}
	if _, bad := forbiddenAES[cfg.Security.AESKey]; bad {
		log.Fatalf("配置校验失败: [security].aes_key 在生产模式下不得为默认值或占位符，请通过环境变量 NETYADMIN_AES_KEY 设置真实密钥")
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
	}
	if cfg.Redis.Enabled {
		if _, bad := forbiddenRedisPwd[cfg.Redis.Password]; bad {
			log.Fatalf("配置校验失败: [redis].password 在生产模式下不得为空或占位符，请通过环境变量 NETYADMIN_REDIS_PASSWORD 设置真实密码")
		}
	}
}
