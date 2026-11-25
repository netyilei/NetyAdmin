// helpers.go：接线帮手（config → 组件参数映射；日志脱敏默认组合；JWT 签发密钥准备）。
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/netyilei/netyadmin/internal/kernel/config"
	"github.com/netyilei/netyadmin/internal/kernel/hooks"
	klog "github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/keys"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
	"github.com/netyilei/netyadmin/internal/kernel/state/session"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/rbac"
	rbacdomain "github.com/netyilei/netyadmin/internal/modules/rbac/domain"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// —— config → 组件参数映射 ——

// topologyOf config.Deployment → coordination 拓扑。
func topologyOf(cfg *config.Config) coordination.Topology {
	if cfg.Deployment.Topology == "multi" {
		return coordination.TopologyMulti
	}
	return coordination.TopologySingle
}

// sessionTopology config.Deployment → session 平面拓扑（identity 模块 session
// 降级语义输入；值与 coordination.Topology 同枚举）。
func sessionTopology(cfg *config.Config) session.Topology {
	if cfg.Deployment.Topology == "multi" {
		return session.TopologyMulti
	}
	return session.TopologySingle
}

// redisAddr 取 Redis 地址（config.Redis.Addr；env NETYADMIN_REDIS_ADDR 经
// envBindings 覆盖同一字段——单一来源）。
func redisAddr(cfg *config.Config) string {
	return cfg.Redis.Addr
}

// coordNamespace coordination 键命名空间（na:{env}:coord——铁律 6 环境段；
// 装配与诊断面共用，防双源漂移）。
func coordNamespace(cfg *config.Config) string {
	return cfg.Redis.Prefix + ":" + cfg.Server.Env + ":coord"
}

// captchaNamespace captcha 键命名空间（na:{env}:captcha——铁律 6 环境段；
// namespaces.yaml kernel_plane_keys 登记；与 coordNamespace 同拼法防漂移）。
func captchaNamespace(cfg *config.Config) string {
	return cfg.Redis.Prefix + ":" + cfg.Server.Env + ":captcha"
}

// sessionNamespace session 平面键命名空间（na:{env}:sess——铁律 6 环境段；
// namespaces.yaml kernel_plane_keys 登记；与 coordNamespace 同拼法防漂移）。
func sessionNamespace(cfg *config.Config) string {
	return cfg.Redis.Prefix + ":" + cfg.Server.Env + ":sess"
}

// cacheNamespace cache 平面键命名空间（na:{env}:cache——铁律 6 环境段；
// namespaces.yaml kernel_plane_keys 登记；与 coordNamespace 同拼法防漂移）。
func cacheNamespace(cfg *config.Config) string {
	return cfg.Redis.Prefix + ":" + cfg.Server.Env + ":cache"
}

// addrOf HTTP 监听地址（flag 覆盖 config）。
func addrOf(port int, flag string) string {
	if flag != "" {
		return flag
	}
	return fmt.Sprintf(":%d", port)
}

// ginMode 运行模式（控制台日志仅开发，logging.md）。
func ginMode(env string) string {
	if env == "dev" {
		return "debug"
	}
	return "release"
}

// logOptions 由配置组装日志出口参数（config → log.Options 唯一映射点）。
// 新增 config.Log 字段必须在此接线——「配置声明但零消费」是静默失效缺陷
// （server.trusted_proxies / log.compress 前例）；wiring_config_test 逐字段对偶。
func logOptions(cfg *config.Config, debugTTL time.Duration) klog.Options {
	return klog.Options{
		Dir:           cfg.Log.Dir,
		Level:         cfg.Log.Level,
		Console:       cfg.Log.Console,
		Format:        cfg.Log.Format,
		NodeID:        cfg.Deployment.NodeID,
		Redact:        logRedactValue,       // 值模式：手机/邮箱（primitives 掩码原语组合）
		RedactKeys:    defaultLogRedactKeys, // 键名黑名单：password/token/secret 等凭据类
		RetentionDays: cfg.Log.RetentionDays,
		MaxSizeMB:     cfg.Log.MaxSizeMB,
		MaxBackups:    cfg.Log.MaxBackups,
		Compress:      cfg.Log.Compress,
		DebugTTL:      debugTTL,
	}
}

// transportOptions 由配置组装 HTTP 传输面参数（config → transport.Options 唯一
// 映射点；addr 已含 -addr flag 覆盖）。BodyLimit/ReadHeaderTO 无配置面——由
// transport 默认值恒定（10MB / 10s）。
// TrustedProxies=反代信任面：空=不信任 X-Forwarded-For（ClientIP 取直连地址）；
// 反代部署必须登记，否则审计/登录限流/IPAC 全部按代理 IP 判定。
func transportOptions(cfg *config.Config, addr string, hookReg *hooks.Registry,
	tracing *observability.Tracing, idem *coordination.Coordination, accessLog *slog.Logger) transport.Options {
	return transport.Options{
		Addr:           addr,
		Mode:           ginMode(cfg.Server.Env),
		Env:            cfg.Server.Env, // prod 才发 HSTS
		CSP:            cfg.Server.CSP, // 空=Report-Only 宽策略默认；非空=enforce
		CORSOrigins:    cfg.Server.CORSOrigins,
		TrustedProxies: cfg.Server.TrustedProxies,
		Hooks:          hookReg,
		Tracing:        tracing,
		Idempotency:    idem,
		AccessLogger:   accessLog,
	}
}

// pingRedis 探活（coordination.Client）。
func pingRedis(c coordination.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.Ping(ctx)
}

// pingRedisCoord 已建协调面的真探活（运行期翻转 ready）。
func pingRedisCoord(ctx context.Context, c *coordination.Coordination) error {
	return c.Ping(ctx)
}

// signalContext SIGINT/SIGTERM → ctx 取消（bootstrap 只认 ctx）。
func signalContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()
	return ctx
}

// reconcileAllowedCallerRoles 对账 config.rpc.allowed_callers 与 rbac_roles.code
// （R3「role×allowed_callers 交叉校验」轻量固化）：白名单登记的调用方
// 服务名若需授权面（角色），应在 rbac 登记同 code 角色——漂移=告警日志不阻断
// （服务名与角色码同为部署登记面，人工核对；fail-fast 随首个误配事故升级）。
func reconcileAllowedCallerRoles(ctx context.Context, m *rbac.Module, callers []string, logger *slog.Logger) error {
	const roleScanCap = 1000 // 对账扫描上限（角色量级远低于此；防漏配护栏）
	roles, _, err := m.Service().ListRoles(ctx, rbacdomain.RoleListFilter{Limit: roleScanCap})
	if err != nil {
		return fmt.Errorf("rbac: list roles for allowed_callers reconcile: %w", err)
	}
	codes := make(map[string]bool, len(roles))
	for _, r := range roles {
		codes[r.Code] = true
	}
	for _, caller := range missingCallerRoles(codes, callers) {
		logger.Warn("rpc allowed_caller 无对应 rbac 角色（部署登记漂移——核对 config.rpc.allowed_callers 与 rbac_roles.code）",
			"caller", caller)
	}
	return nil
}

// missingCallerRoles 白名单条目不在角色 code 集合中的清单（纯函数——测试面）。
func missingCallerRoles(codes map[string]bool, callers []string) []string {
	var missing []string
	for _, c := range callers {
		if !codes[c] {
			missing = append(missing, c)
		}
	}
	return missing
}

// —— 日志出口脱敏 ——
// 日志出口脱敏默认组合（组合根面——kernel/log 保持机制零预设）。
//
// 两条互补通道（logging.md §6「脱敏：复用 mask/PII；禁止 raw body、密钥」）：
//   - 值模式：Redact 扫 msg/属性值中的手机号与邮箱，按 primitives 掩码原语改写
//     （自由文本里无法按键名识别，只能按模式兜底）；
//   - 键名黑名单：RedactKeys 对凭据类键（password/token/secret 等）整值 [redacted]
//     ——这类值无稳定模式，只能按键名 fail-closed。

// defaultLogRedactKeys 是键名黑名单基线（含常见变体；log 包按精确/下划线/连字符
// 后缀匹配，old_password/new-password 同命中）。
var defaultLogRedactKeys = []string{
	"password", "passwd", "old_password", "new_password",
	"token", "access_token", "refresh_token", "authorization",
	"secret", "client_secret", "app_secret", "private_key",
	"phone", "mobile", "email",
}

var (
	// 中国大陆手机号（11 位，1[3-9] 前缀；词边界避免长数字串误伤）。
	logPhonePattern = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	// 邮箱（宽松 RFC 5322 子集——覆盖日志常见形态即可，不求完整校验）。
	logEmailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
)

// logRedactValue 是 log.Options.Redact 默认实现：手机号/邮箱按 primitives 掩码。
func logRedactValue(s string) string {
	if s == "" {
		return s
	}
	s = logPhonePattern.ReplaceAllStringFunc(s, primitives.MaskPhone)
	s = logEmailPattern.ReplaceAllStringFunc(s, primitives.MaskEmail)
	return s
}

// —— JWT 签发密钥 ——
// JWT 签发密钥准备——dev 首启自动生成写文件（/keys/
// 已 gitignore）；prod 缺文件 fail-closed 报错指引（security.md §1 部署契约）。

// keysDir 是密钥目录（AGENTS §14 本地环境；.gitignore 锚定 /keys/）。
const keysDir = "keys"

// ensureJWTKeys 装载或（dev 下）生成签发密钥集。
func ensureJWTKeys(cfg *config.Config) (*keys.SigningKeys, error) {
	if err := os.MkdirAll(keysDir, 0o700); err != nil {
		return nil, fmt.Errorf("keys: mkdir: %w", err)
	}
	if _, err := keys.Load(keysDir); err == nil {
		return keys.Load(keysDir)
	} else if !strings.Contains(err.Error(), filepath.Join("keys", keys.FileNameActive)) {
		// 非「active 缺失」（破损/格式错）→ fail-closed 不覆盖
		return nil, err
	}
	if cfg.Server.Env == "prod" {
		return nil, fmt.Errorf(
			"keys: %s missing — provide signing key material before deploying to prod (security.md §1)",
			filepath.Join(keysDir, keys.FileNameActive))
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("keys: generate: %w", err)
	}
	if err := os.WriteFile(filepath.Join(keysDir, keys.FileNameActive),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}),
		0o600); err != nil {
		return nil, fmt.Errorf("keys: write active: %w", err)
	}
	return keys.Load(keysDir)
}
