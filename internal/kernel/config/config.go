// Package config 提供分层配置：默认值 → 文件（YAML）→ 环境变量 → 动态配置（configcenter）→ Secrets。
//
// 分层与校验语义：
//   - 后层覆盖前层；env 走显式映射表（NETYADMIN_<段>_<字段>），不用反射；
//   - Secrets 只从环境变量注入，永不入文件；生产密钥占位符 fail-closed（security.md §1）；
//   - state.mode=auto 仅开发环境（staging/prod 拒绝）；multi 拓扑要求 Redis 显式配置；
//   - 未知配置段启动期告警（不阻断，诊断可见）。
//
// 模块配置段绑定（Manifest config_section + schema）依赖模块注册表，
// 在落地时接入本包，不在范围。
//
// 热更边界（docs/rules/hot-update.md）：监听地址/DSN/Redis 地址/模块启停不热更；
// 动态配置（configcenter） 接入时经 LoadOptions 扩展，不改变本包分层语义。
// 实现：（分层/密钥/校验）。
package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// —— 段定义 ——

// Server 段：进程与接入面端口。
type Server struct {
	// TrustedProxies 是反向代理 CIDR 白名单（空=不信任代理头——本地直连默认）。
	TrustedProxies []string `yaml:"trusted_proxies"`
	Env            string   `yaml:"env"` // dev | staging | prod
	HTTPPort       int      `yaml:"http_port"`
	RPCPort        int      `yaml:"rpc_port"`
	// OpsPprof 是 /debug/pprof 暴露开关（OPS-01；默认关——生产 fail-closed，
	// 开发/诊断期显式开启；/health 与 /metrics 不受此开关影响——LB 探针面）。
	OpsPprof bool `yaml:"ops_pprof"`
	// CSP 是 Content-Security-Policy 策略串（安全响应头项 7）。空=默认发
	// Content-Security-Policy-Report-Only 宽策略（SPA/PWA 不破坏，渐进收紧）；
	// 非空=发 Content-Security-Policy 强制 enforce。HSTS 由 env=prod 恒发
	// （无独立开关——传输安全声明不配策略面）。
	CSP string `yaml:"csp"`
	// CORSOrigins 是浏览器跨域白名单（精确 Origin；空=不启用 CORS——现状安全
	// 语义）。显式登记才开（禁 "*"——与 Allow-Credentials 组合非法，validate 拒启）。
	CORSOrigins []string `yaml:"cors_origins"`
}

// Deployment 段：部署拓扑（AGENTS §3）。
type Deployment struct {
	Topology string `yaml:"topology"` // single | multi
	// Role 是本进程装配角色（部署形态；预设的子集）：
	// all=全接入面（默认单机）；core=平台服务+RPC 宿主（无 admin/client HTTP 门）；
	// admin=接入面+RPC 宿主（无 client 门）。模块全装配（共享单库——模块裁剪
	// 随拆库才真实）；role 只过滤接入面挂载。
	Role string `yaml:"role"` // all | core | admin
	// ServiceName 是本进程服务标识（内部 RPC 调用方 JWT sub；
	// 部署形态登记面；空=local 单进程（不发起 remote RPC））。
	ServiceName string `yaml:"service_name"`
	// NodeID 是本进程节点标识（全集群唯一——日志 node_id/诊断、Bus consumer、
	// scheduler 分片、GW 统计快照文件名共用；multi-node-scaling §8「各副本配置
	// 一致（node_id 除外）」）。默认 node-1（单机）；multi 每副本显式覆盖。
	NodeID string `yaml:"node_id"`
}

// State 段：状态子系统模式（state-and-cache.md；auto 仅开发）。
type State struct {
	Mode string `yaml:"mode"` // strict | auto
}

// GW 段：网关治理——App secret AES 加密钥（Secret 注入）。
type GW struct {
	// SecretKey 是 App secret 加密钥（base64 解码后必须 32 字节——AES-256）。
	// Secret 面 env-only（分层；NETYADMIN_GW_SECRET_KEY）。
	SecretKey Secret `yaml:"-"` // 只从 env 注入
}

// Realtime 段：WS 限制参数（ws-protocol.md §6「所有限制必须通过配置」
// ——默认值即规范基线，部署可覆盖）。
//
// 单一事实源：realtime 模块运行面直接消费本结构（kernel/config.Realtime 注入
// module.New），**禁止**模块再声明同名配置结构——旧模块 Config + ConfigBinder
// 双结构体只绑 YAML 显式字段、defaults/env 全丢，导致三限制（单 App 连接/
// 单身份连接/App 维度限流）=0 静默失效（HV-1/HV-4 同根）。
type Realtime struct {
	// Enabled 是 WS 面总开关（false=三端 upgrade 拒绝——禁用模块零开销）。
	Enabled bool `yaml:"enabled"`
	// HeartbeatInterval 心跳 ping 周期（秒；默认 30）。
	HeartbeatInterval int `yaml:"heartbeat_interval"`
	// IdleTimeout 空闲超时（秒；无 pong 关闭 1001；默认 90）。
	IdleTimeout int `yaml:"idle_timeout"`
	// SendQueue 单连接发送队列（默认 64——ws-protocol §6 规范值）。
	SendQueue int `yaml:"send_queue"`
	// MaxConnsPerPrincipal 同身份连接上限（默认 5；超出 4005；user/admin/
	// service 型——按 principal.Type 分型计数）。
	MaxConnsPerPrincipal int `yaml:"max_conns_per_principal"`
	// MaxConnsPerApp 单 App 连接上限（默认 10000；超出 1013；app principal
	// 独立计数——ws-protocol §6 单 App 行）。
	MaxConnsPerApp int `yaml:"max_conns_per_app"`
	// MaxConnsTotal 节点级全局连接上限（默认 100000；0=不限显式声明；
	// 资源上限红线——防多 App 连接累加耗尽节点内存）
	MaxConnsTotal int `yaml:"max_conns_total"`
	// MaxChannelsPerConn 单连接频道数上限（默认 100）。
	MaxChannelsPerConn int `yaml:"max_channels_per_conn"`
	// MaxFrameBytes 单帧大小上限（默认 64KB；超出 4004）。
	MaxFrameBytes int `yaml:"max_frame_bytes"`
	// AdminOrigins 是 admin surface 浏览器 Origin 白名单（空=拒所有带 Origin
	// 请求——非浏览器客户端直连； 前端接入前常为空）。
	AdminOrigins []string `yaml:"admin_origins"`
	// ClientOrigins 是 client surface 浏览器 Origin 白名单（H5/小程序 webview
	// 等浏览器形态 C 端客户端；空=拒所有带 Origin 请求——App/SDK 等非浏览器
	// 客户端无 Origin 可直连）。gateway 面无此配置：AppSecret 不进浏览器，
	// 第三方 WS 只能由服务端签名接入。
	ClientOrigins []string `yaml:"client_origins"`
	// MessageRatePerSec 每连接消息速率（默认 10/s；超出 4003）。
	MessageRatePerSec int `yaml:"message_rate_per_sec"`
	// MaxMessageRatePerAppSec 单 App 上行消息速率（默认 1000/s；gateway app
	// principal 应用维度滑窗——策略名 gateway_app 与 HTTP 面一致；超出 4003）。
	MaxMessageRatePerAppSec int `yaml:"max_message_rate_per_app_sec"`
	// RequireHTTPS 是 WS upgrade 的 X-Forwarded-Proto 校验开关（默认 false；
	// prod 强制 true——Load 归一，见 normalize）。边缘终结语义：
	// prod 下 TLS 由网关/反代终结，进程内以 X-Forwarded-Proto: http 兜底拒绝
	// 明文 upgrade（否则 token 走明文 WS 暴露）；dev/staging 明文联调不拦。
	RequireHTTPS bool `yaml:"require_https"`
}

// Users 段：users 演示模块开关（US-01 模块开关演示——false=零注册零开销）。
type Users struct {
	Enabled    bool `yaml:"enabled"`
	BcryptCost int  `yaml:"bcrypt_cost"` // 默认 12（security.md §2 成本参数进配置）
}

// Message 段：通知发送驱动选择（驱动模式——email=smtp 内置/sms=log
// 默认；厂商驱动下游自写 Register 注册；smtp password 走 env Secret）。
type Message struct {
	// Enabled 是模块开关（——false=零注册零开销；迁移恒登记=部署期建表，
	// 开关只控制路由/Job/goroutine——users 先例）。
	Enabled     bool       `yaml:"enabled"`
	EmailDriver string     `yaml:"email_driver"` // 默认 smtp
	SmsDriver   string     `yaml:"sms_driver"`   // 默认 log
	SMTP        SMTPConfig `yaml:"smtp"`
	// SMTPPassword 是 SMTP 认证密码（Secret 面 env-only：
	// NETYADMIN_MESSAGE_SMTP_PASSWORD—— 分层）。
	SMTPPassword Secret `yaml:"-"`
}

// SMTPConfig 是 SMTP 驱动参数（password env-only：NETYADMIN_MESSAGE_SMTP_PASSWORD）。
type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	From     string `yaml:"from"`
	// Password 运行期注入（secretFromEnv——非 yaml 序列化面）。
	Password string `yaml:"-"`
}

// Storage 段：对象存储模块（——HMAC 盐 env Secret+凭证 TTL）。
type Storage struct {
	Enabled               bool   `yaml:"enabled"`                 //  模块开关（默认 true）
	CredentialsTTLSeconds int    `yaml:"credentials_ttl_seconds"` // 默认 900（15min）
	HMACSalt              Secret `yaml:"-"`                       // env-only NETYADMIN_STORAGE_HMAC_SALT
}

// Cache 段：Cache 平面运行参数（env-only——无 config.yaml 文件面）。
type Cache struct {
	// L2EncryptKey 是 classification=pii 实体 L2 值加密钥（base64 解码后必须
	// 32 字节——AES-256-GCM；contracts cache.pii.encrypt_l2）。Secret 面
	// env-only（NETYADMIN_CACHE_L2_KEY）；缺失=pii 实体注册失败（fail-closed
	// ——禁明文落共享 Redis）。非 pii 实体不受影响。
	L2EncryptKey Secret `yaml:"-"`
}

// RPC 段：内部 RPC 宿主（「不信任内网」——接收方白名单显式登记）。
type RPC struct {
	// AllowedCallers 是合法调用方服务名（JWT sub）清单；空=RPC 宿主仍挂载但
	// 全拒 fail-closed（显式登记才可调——部署拓扑登记面）。
	AllowedCallers []string `yaml:"allowed_callers"`
}

// BindingEntry 是一个端口的绑定声明（/；——base_urls 空=
// local 同进程直调；非空=remote 经内部 RPC 轮询 LB）。
type BindingEntry struct {
	Port     string   `yaml:"port"`      // Registry 端口名（如 rbac.Authorizer）
	BaseURLs []string `yaml:"base_urls"` // 目标实例清单
}

// Database 段：PostgreSQL 连接参数（密码走 Secret）。
type Database struct {
	// ReplicaDSN 是只读副本 DSN（读写分离；空=无副本——读也走主）。
	ReplicaDSN string `yaml:"replica_dsn"`
	// MaxConns 是主连接池上限（0=pgxpool 库默认 max(4,NumCPU)——对齐上游默认，
	// 不擅自改语义；多实例/拆库部署按进程数×副本数显式规划，装-A3）。
	MaxConns int    `yaml:"max_conns"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	SSLMode  string `yaml:"sslmode"`
	Password Secret `yaml:"-"` // 只从 env 注入
}

// Redis 段：连接与命名前缀。
type Redis struct {
	Addr   string `yaml:"addr"`
	DB     int    `yaml:"db"`
	Prefix string `yaml:"prefix"`
}

// Log 段：日志目录与级别（logging.md §5 全字段）。
type Log struct {
	Dir           string `yaml:"dir"`
	Level         string `yaml:"level"`          // debug|info|warn|error；debug 默认关
	Format        string `yaml:"format"`         // json|text（开发 text；生产 json——§5）
	RetentionDays int    `yaml:"retention_days"` // 按日目录保留天数（lumberjack MaxAge + 目录清扫）
	MaxSizeMB     int    `yaml:"max_size_mb"`    // 单文件大小上限（MB；默认 100）
	MaxBackups    int    `yaml:"max_backups"`    // 备份份数（0=不限——按日目录组织）
	Compress      bool   `yaml:"compress"`       // 轮转备份 gzip 压缩（默认关——§5）
	DebugTTL      string `yaml:"debug_ttl"`      // debug 自动恢复时长（如 30m；空=不启用——§5）
	Console       bool   `yaml:"console"`
}

// Observability 段：追踪接线（observability.md §4）与观测页告警阈值。
// 启动期配置——endpoint 不热更（hot-update 边界）；本地无 collector时 Enabled=false 零开销。
type Observability struct {
	Tracing TracingConfig `yaml:"tracing"`
	// Alerts 是观测页阈值覆盖（键见 kernel/observability.DefaultThresholds；
	// 空=全部用默认值。展示型告警：页面请求时评估，不落库/不推送）。
	Alerts map[string]float64 `yaml:"alerts"`
}

// TracingConfig 是追踪子段。
type TracingConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Endpoint    string  `yaml:"endpoint"`     // OTLP/HTTP collector（如 localhost:4318）
	SampleRatio float64 `yaml:"sample_ratio"` // [0,1]；未配置走默认 1.0（开发全采，生产建议 0.1）；显式 0=全不采样
	ServiceName string  `yaml:"service_name"` // resource service.name；空=默认 netyadmin
}

// Config 是启动期解析完成的只读快照；运行期不重新解释（热更边界见包注释）。
type Config struct {
	Server        Server        `yaml:"server"`
	Deployment    Deployment    `yaml:"deployment"`
	State         State         `yaml:"state"`
	Database      Database      `yaml:"database"`
	Redis         Redis         `yaml:"redis"`
	Log           Log           `yaml:"log"`
	Observability Observability `yaml:"observability"`
	RPC           RPC           `yaml:"rpc"`

	Message  Message        `yaml:"message"`
	Storage  Storage        `yaml:"storage"`
	GW       GW             `yaml:"gw"`
	Users    Users          `yaml:"users"`
	Realtime Realtime       `yaml:"realtime"`
	Binding  []BindingEntry `yaml:"binding"`
	Cache    Cache          `yaml:"-"` // env-only（NETYADMIN_CACHE_L2_KEY）

	sections map[string]any // 原始段（模块 ConfigBinder 绑定用）
}

// Section 返回指定原始配置段（启动期冻结的只读快照，调用方不得修改；模块 ConfigBinder 绑定目标）。
// 不存在的段返回 false——模块声明 config_section 前先确认配置文件提供了它。
func (c *Config) Section(name string) (map[string]any, bool) {
	raw, ok := c.sections[name]
	if !ok {
		return nil, false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return map[string]any{}, true // 段存在但为标量/空
	}
	return m, true
}

// HasSection 段是否存在。
func (c *Config) HasSection(name string) bool {
	_, ok := c.sections[name]
	return ok
}

// Diagnostics 返回生效配置诊断（§10：当前生效值脱敏展示；诊断数据面 config 用，
//）。Secret 永不入返回值（String/GoString 双掩码口径兜底）；
// 字段级来源/变更时间随 configcenter 接入补齐。
func (c *Config) Diagnostics() map[string]any {
	db := c.Database
	return map[string]any{
		"server":     c.Server,
		"deployment": c.Deployment,
		"rpc":        c.RPC,
		"binding":    c.Binding,
		"gw":         c.GW,
		"realtime":   c.Realtime,
		"state":      c.State,
		"database": map[string]any{
			"host": db.Host, "port": db.Port, "name": db.Name, "user": db.User,
			"sslmode": db.SSLMode, "password_set": db.Password.IsSet(),
		},
		"redis":         c.Redis,
		"log":           c.Log,
		"observability": c.Observability,
	}
}

// knownSections 用于未知段告警；新增段必须同步此处与 envBindings。
// 含两块：内核段（本包结构体消费）+ 模块 config_section 段（经 sections map
// 绑定）。模块段清单是各模块 module.yaml config_section 的镜像——config 包
// 不能 import modules（依赖方向：kernel 不得反向依赖业务），故维护静态名单；
// **新增/改名模块 config_section 时必须同步此处**（config_test 有 module.yaml
// 全量对偶断言，漏同步即红灯）。未知段告警不阻断。
var knownSections = []string{
	// 内核段
	"server", "deployment", "state", "database", "redis", "log", "observability",
	"rpc", "gw", "audit", "realtime", "binding", "message", "users", "storage",
	// 模块 config_section（对齐 internal/modules/*/module.yaml）
	"identity", "rbac", "configcenter", "note",
}

// —— 错误（领域错误；接口状态码映射随在调用方完成） ——

var (
	ErrInvalidValue      = errors.New("config: invalid value")
	ErrSecretPlaceholder = errors.New("config: secret is placeholder or unset")
)

// —— 加载 ——

// LoadOptions 控制加载来源；零值即默认行为。
type LoadOptions struct {
	// Path 是 YAML 配置文件（可选；空=只用默认值+env）。缺失文件视为错误。
	Path string
	// Environ 覆盖进程环境（测试注入用）；nil 时用 os.Environ。
	Environ []string
}

// Load 解析并校验配置；返回只读快照、未知段告警与汇总错误（errors.Join，含全部违规项）。
// 错误非 nil 时 cfg 为 nil（调用方必须 fail-closed）。
func Load(opts LoadOptions) (*Config, []string, error) {
	cfg := defaults()

	warnings := []string{}
	var errs []error

	if opts.Path != "" {
		raw, err := os.ReadFile(opts.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("config: read file: %w", err)
		}
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, nil, fmt.Errorf("config: parse yaml: %w", err)
		}
		if err := yaml.Unmarshal(raw, &cfg.sections); err != nil {
			return nil, nil, fmt.Errorf("config: parse yaml: %w", err)
		}
		warnings = append(warnings, unknownSections(raw)...)
	}

	for _, b := range envBindings {
		v := lookupEnv(opts.Environ, b.key)
		if v == "" {
			continue
		}
		if err := b.set(cfg, v); err != nil {
			errs = append(errs, fmt.Errorf("config: env %s: %w", b.key, err))
		}
	}

	cfg.Database.Password = secretFromEnv(opts.Environ, "NETYADMIN_DATABASE_PASSWORD")
	cfg.GW.SecretKey = secretFromEnv(opts.Environ, "NETYADMIN_GW_SECRET_KEY")
	cfg.Message.SMTPPassword = secretFromEnv(opts.Environ, "NETYADMIN_MESSAGE_SMTP_PASSWORD")
	cfg.Storage.HMACSalt = secretFromEnv(opts.Environ, "NETYADMIN_STORAGE_HMAC_SALT")
	cfg.Cache.L2EncryptKey = secretFromEnv(opts.Environ, "NETYADMIN_CACHE_L2_KEY")

	cfg.normalize()
	errs = append(errs, cfg.validate()...)
	if err := errors.Join(errs...); err != nil {
		return nil, warnings, err
	}
	return cfg, warnings, nil
}

// normalize 收敛跨层强制语义（不报错、只改写）——当前唯一项：
// prod 强制 realtime.require_https=true（明文 WS 暴露 token 的进程内兜底；
// 配置面 default false、prod 覆盖优先——显式配置在 dev/staging 有效）。
func (c *Config) normalize() {
	if c.Server.Env == "prod" {
		c.Realtime.RequireHTTPS = true
	}
}

func defaults() *Config {
	return &Config{
		Server:     Server{Env: "dev", HTTPPort: 8080, RPCPort: 9090, CSP: ""}, // CSP 空=Report-Only 宽策略默认
		Deployment: Deployment{Topology: "single", Role: "all", NodeID: "node-1"},
		State:      State{Mode: "strict"},
		Database:   Database{Host: "127.0.0.1", Port: 5432, Name: "netyadmin_v2", User: "netyadmin", SSLMode: "disable"},
		Redis:      Redis{Addr: "127.0.0.1:6379", DB: 0, Prefix: "na"},
		Log: Log{Dir: "logs", Level: "info", Console: true, Format: "text",
			RetentionDays: 30, MaxSizeMB: 100},
		Observability: Observability{Tracing: TracingConfig{Enabled: false, SampleRatio: 1.0}},
		RPC:           RPC{},
		Message:       Message{Enabled: true, EmailDriver: "smtp", SmsDriver: "log", SMTP: SMTPConfig{Port: 587}},
		Users:         Users{Enabled: false, BcryptCost: 12},
		Storage:       Storage{Enabled: true, CredentialsTTLSeconds: 900},
		Realtime: Realtime{HeartbeatInterval: 30, IdleTimeout: 90, SendQueue: 64,
			MaxConnsPerPrincipal: 5, MaxConnsPerApp: 10000, MaxConnsTotal: 100000, MaxChannelsPerConn: 100,
			MaxFrameBytes: 64 * 1024, MessageRatePerSec: 10, MaxMessageRatePerAppSec: 1000,
			RequireHTTPS: false}, // 默认 false；prod 由 normalize 强制 true
	}
}

// —— env 显式映射表 ——

type envBinding struct {
	key string
	set func(*Config, string) error
}

var envBindings = []envBinding{
	{"NETYADMIN_ENV", func(c *Config, v string) error { return setEnum(&c.Server.Env, v, "dev", "staging", "prod") }},
	{"NETYADMIN_SERVER_HTTP_PORT", func(c *Config, v string) error { return setPort(&c.Server.HTTPPort, v) }},
	{"NETYADMIN_SERVER_RPC_PORT", func(c *Config, v string) error { return setPort(&c.Server.RPCPort, v) }},
	{"NETYADMIN_SERVER_CSP", func(c *Config, v string) error { c.Server.CSP = v; return nil }},
	{"NETYADMIN_DEPLOYMENT_TOPOLOGY", func(c *Config, v string) error { return setEnum(&c.Deployment.Topology, v, "single", "multi") }},
	{"NETYADMIN_DEPLOYMENT_ROLE", func(c *Config, v string) error { return setEnum(&c.Deployment.Role, v, "all", "core", "admin") }},
	{"NETYADMIN_DEPLOYMENT_NODE_ID", func(c *Config, v string) error { c.Deployment.NodeID = v; return nil }},
	{"NETYADMIN_STATE_MODE", func(c *Config, v string) error { return setEnum(&c.State.Mode, v, "strict", "auto") }},
	{"NETYADMIN_DATABASE_HOST", func(c *Config, v string) error { c.Database.Host = v; return nil }},
	{"NETYADMIN_DATABASE_PORT", func(c *Config, v string) error { return setPort(&c.Database.Port, v) }},
	{"NETYADMIN_DATABASE_NAME", func(c *Config, v string) error { c.Database.Name = v; return nil }},
	{"NETYADMIN_DATABASE_USER", func(c *Config, v string) error { c.Database.User = v; return nil }},
	{"NETYADMIN_DATABASE_SSLMODE", func(c *Config, v string) error { c.Database.SSLMode = v; return nil }},
	{"NETYADMIN_DATABASE_MAX_CONNS", func(c *Config, v string) error { return setRange(&c.Database.MaxConns, v, 1, 10000) }},
	{"NETYADMIN_REDIS_ADDR", func(c *Config, v string) error { c.Redis.Addr = v; return nil }},
	{"NETYADMIN_REDIS_DB", func(c *Config, v string) error { return setRange(&c.Redis.DB, v, 0, 15) }},
	{"NETYADMIN_REDIS_PREFIX", func(c *Config, v string) error { c.Redis.Prefix = v; return nil }},
	{"NETYADMIN_LOG_DIR", func(c *Config, v string) error { c.Log.Dir = v; return nil }},
	{"NETYADMIN_LOG_LEVEL", func(c *Config, v string) error { return setEnum(&c.Log.Level, v, "debug", "info", "warn", "error") }},
	{"NETYADMIN_LOG_CONSOLE", func(c *Config, v string) error { return setBool(&c.Log.Console, v) }},
	{"NETYADMIN_LOG_FORMAT", func(c *Config, v string) error { return setEnum(&c.Log.Format, v, "json", "text") }},
	{"NETYADMIN_OBSERVABILITY_TRACING_ENABLED", func(c *Config, v string) error { return setBool(&c.Observability.Tracing.Enabled, v) }},
	{"NETYADMIN_OBSERVABILITY_TRACING_ENDPOINT", func(c *Config, v string) error { c.Observability.Tracing.Endpoint = v; return nil }},
	{"NETYADMIN_OBSERVABILITY_TRACING_SAMPLE_RATIO", func(c *Config, v string) error { return setRatio(&c.Observability.Tracing.SampleRatio, v) }},
	{"NETYADMIN_OBSERVABILITY_TRACING_SERVICE_NAME", func(c *Config, v string) error { c.Observability.Tracing.ServiceName = v; return nil }},
	{"NETYADMIN_REALTIME_MAX_CONNS_PER_APP", func(c *Config, v string) error { return setRange(&c.Realtime.MaxConnsPerApp, v, 1, 10000000) }},
	{"NETYADMIN_REALTIME_MAX_CONNS_TOTAL", func(c *Config, v string) error { return setRange(&c.Realtime.MaxConnsTotal, v, 0, 10000000) }},
	{"NETYADMIN_REALTIME_MAX_MESSAGE_RATE_PER_APP_SEC", func(c *Config, v string) error { return setRange(&c.Realtime.MaxMessageRatePerAppSec, v, 1, 100000000) }},
}

func lookupEnv(environ []string, key string) string {
	if environ == nil {
		return os.Getenv(key)
	}
	prefix := key + "="
	for _, kv := range environ {
		if strings.HasPrefix(kv, prefix) {
			return kv[len(prefix):]
		}
	}
	return ""
}

func secretFromEnv(environ []string, key string) Secret {
	if v := lookupEnv(environ, key); v != "" {
		return NewSecret(v, "env:"+key)
	}
	return Secret{}
}

// —— setter 帮手 ——

func setEnum(dst *string, v string, allowed ...string) error {
	for _, a := range allowed {
		if v == a {
			*dst = v
			return nil
		}
	}
	return fmt.Errorf("%w: %q 不在允许值 %v", ErrInvalidValue, v, allowed)
}

func setPort(dst *int, v string) error {
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("%w: %q 非法端口", ErrInvalidValue, v)
	}
	*dst = n
	return nil
}

func setRange(dst *int, v string, lo, hi int) error {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("%w: %q 非整数", ErrInvalidValue, v)
	}
	if n < lo || n > hi {
		return fmt.Errorf("%w: %q 超出范围 [%d,%d]", ErrInvalidValue, v, lo, hi)
	}
	*dst = n
	return nil
}

func setBool(dst *bool, v string) error {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fmt.Errorf("%w: %q 非布尔", ErrInvalidValue, v)
	}
	*dst = b
	return nil
}

// —— 校验与告警 ——

func (c *Config) validate() []error {
	var errs []error
	prod := c.Server.Env == "prod"

	if prod && c.Database.Password.IsPlaceholder() {
		errs = append(errs, fmt.Errorf("%w: NETYADMIN_DATABASE_PASSWORD（生产 fail-closed，security.md §1）", ErrSecretPlaceholder))
	}
	if c.State.Mode == "auto" && c.Server.Env != "dev" {
		errs = append(errs, fmt.Errorf("%w: state.mode=auto 仅开发可用（staging/prod 必须 strict，state-and-cache.md）", ErrInvalidValue))
	}
	if c.Deployment.Topology == "multi" && c.Redis.Addr == "" {
		errs = append(errs, fmt.Errorf("%w: multi 拓扑必须配置 Redis（AGENTS §3/§5）", ErrInvalidValue))
	}
	// node_id：全集群唯一标识（multi 每副本不同）。字符集收窄到文件/键安全面
	// （GW 统计快照文件名直用；小写字母数字与 ._-——禁路径分隔/空白/大写）。
	if err := validateNodeID(c.Deployment.NodeID); err != nil {
		errs = append(errs, err)
	}
	// CORS 白名单：禁通配符（"*" 与 Allow-Credentials 组合非法——中间件恒发
	// 凭证头；显式 Origin 才放行，空=不启用 CORS）。
	for _, o := range c.Server.CORSOrigins {
		if o == "*" || strings.Contains(o, "*") {
			errs = append(errs, fmt.Errorf("%w: server.cors_origins 禁止通配符 %q（显式 Origin）", ErrInvalidValue, o))
		}
	}
	if t := c.Observability.Tracing; t.Enabled {
		// fail-closed：开了追踪但没有导出目的地 = 配置非法（span 有采样无导出即静默丢失）
		if t.Endpoint == "" {
			errs = append(errs, fmt.Errorf("%w: observability.tracing.enabled=true 需配置 endpoint", ErrInvalidValue))
		}
		if r := t.SampleRatio; r < 0 || r > 1 {
			errs = append(errs, fmt.Errorf("%w: observability.tracing.sample_ratio %v 超出 [0,1]", ErrInvalidValue, r))
		}
	}
	// 观测页阈值：负数/非有限值非法（阈值语义=超过即告警；0 对 DLQ 类=大于 0
	// 即告警；NaN/Inf 会破坏 JSON 序列化——JSON 拒绝 Inf/NaN）。
	for k, v := range c.Observability.Alerts {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			errs = append(errs, fmt.Errorf("%w: observability.alerts.%s=%v 必须为有限数值", ErrInvalidValue, k, v))
			continue
		}
		if v < 0 {
			errs = append(errs, fmt.Errorf("%w: observability.alerts.%s=%v 必须 ≥0", ErrInvalidValue, k, v))
		}
	}
	// realtime 限制面（ws-protocol §6「所有限制必须通过配置」）：defaults 恒 >0，
	// 显式 0/负数=限制静默失效（HV-1 事故形态）——配置非法拒启，不做运行期兜底。
	limits := []struct {
		name string
		val  int
	}{
		{"heartbeat_interval", c.Realtime.HeartbeatInterval},
		{"idle_timeout", c.Realtime.IdleTimeout},
		{"send_queue", c.Realtime.SendQueue},
		{"max_conns_per_principal", c.Realtime.MaxConnsPerPrincipal},
		{"max_conns_per_app", c.Realtime.MaxConnsPerApp},
		{"max_channels_per_conn", c.Realtime.MaxChannelsPerConn},
		{"max_frame_bytes", c.Realtime.MaxFrameBytes},
		{"message_rate_per_sec", c.Realtime.MessageRatePerSec},
		{"max_message_rate_per_app_sec", c.Realtime.MaxMessageRatePerAppSec},
	}
	for _, l := range limits {
		if l.val <= 0 {
			errs = append(errs, fmt.Errorf("%w: realtime.%s=%d 必须 >0（ws-protocol §6 限制必须生效）", ErrInvalidValue, l.name, l.val))
		}
	}
	return errs
}

// nodeIDMaxLen 是 node_id 长度上限（远超实际命名需要；防滥用长串进文件名/键）。
const nodeIDMaxLen = 64

// validateNodeID 校验节点标识：非空、≤64、首字符字母数字、其余 [a-z0-9._-]。
// 收窄字符集=文件/键/日志安全（GW 统计快照文件名直嵌 node_id）。
func validateNodeID(id string) error {
	if id == "" || len(id) > nodeIDMaxLen {
		return fmt.Errorf("%w: deployment.node_id 长度必须 1..%d", ErrInvalidValue, nodeIDMaxLen)
	}
	for i, r := range id {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if i > 0 {
			ok = ok || r == '.' || r == '_' || r == '-'
		}
		if !ok {
			return fmt.Errorf("%w: deployment.node_id %q 只允许 [a-z0-9._-] 且首字符字母数字", ErrInvalidValue, id)
		}
	}
	return nil
}

// setRatio 解析 [0,1] 浮点（仅合法性；0=显式全不采样，未配置走默认 1.0）。
func setRatio(dst *float64, v string) error {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fmt.Errorf("%w: %q 非数值", ErrInvalidValue, v)
	}
	*dst = f
	return nil
}

// unknownSections 扫描原始 YAML 的顶层键，对未登记段告警（未知段告警不阻断）。
func unknownSections(raw []byte) []string {
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil // 解析错误由主流程报错
	}
	known := make(map[string]bool, len(knownSections))
	for _, s := range knownSections {
		known[s] = true
	}
	var out []string
	for k := range m {
		if !known[k] {
			out = append(out, "未知配置段: "+k)
		}
	}
	return out
}
