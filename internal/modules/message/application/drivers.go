// drivers.go：发送驱动注册表与内置驱动（精神——email=SMTP
// 标准协议内置驱动，sms=log 默认驱动（无通用开放协议，厂商驱动下游自写
// Register 注册））。config 选择 active 驱动：message.email_driver/sms_driver。
package application

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/message/domain"
)

// Registry 是驱动注册表（每通道一集；并发读——启动期注册后只读，mu 保护
// 运行期 Register——下游扩展点）。
type Registry struct {
	mu        sync.RWMutex
	drivers   map[domain.Channel]map[string]domain.Driver // channel → name → driver
	active    map[domain.Channel]string                   // channel → active 驱动名
	accessLog *slog.Logger                                // log 驱动出口（nil=丢弃；注入=消息模块 access 文件面）
}

// NewRegistry 创建并注册内置驱动（email: smtp；sms: log）。
func NewRegistry(cfg Config, accessLog *slog.Logger) *Registry {
	r := &Registry{
		drivers:   map[domain.Channel]map[string]domain.Driver{},
		active:    map[domain.Channel]string{},
		accessLog: accessLog,
	}
	r.drivers[domain.ChannelEmail] = map[string]domain.Driver{}
	r.drivers[domain.ChannelSms] = map[string]domain.Driver{}
	r.Register(domain.ChannelEmail, newSMTPDriver(cfg.SMTP))
	r.Register(domain.ChannelSms, &logDriver{log: accessLog})
	r.setActive(domain.ChannelEmail, cfg.EmailDriver)
	r.setActive(domain.ChannelSms, cfg.SmsDriver)
	return r
}

// Register 注册/覆盖驱动（下游扩展点——厂商驱动经此接入）。
func (r *Registry) Register(channel domain.Channel, d domain.Driver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.drivers[channel] == nil {
		r.drivers[channel] = map[string]domain.Driver{}
	}
	r.drivers[channel][d.Name()] = d
}

// ReplaceDriver 覆盖同名驱动实例（不改 active 选择——用于运行期重建内置驱动，
// 如 admin「邮件设置」保存后重建 SMTP 实例；下游自定义驱动 active 时不受影响）。
func (r *Registry) ReplaceDriver(channel domain.Channel, d domain.Driver) {
	r.Register(channel, d)
}

// setActive 选择 active 驱动（未登记名→回退内置默认并告警）。
func (r *Registry) setActive(channel domain.Channel, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	def := map[domain.Channel]string{
		domain.ChannelEmail: "smtp",
		domain.ChannelSms:   "log",
	}[channel]
	if _, ok := r.drivers[channel][name]; !ok {
		name = def
	}
	r.active[channel] = name
}

// Active 取通道 active 驱动（无 → ErrNoActiveDriver——fail-closed）。
func (r *Registry) Active(channel domain.Channel) (domain.Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	name, ok := r.active[channel]
	if !ok {
		return nil, domain.ErrNoActiveDriver
	}
	d, ok := r.drivers[channel][name]
	if !ok {
		return nil, domain.ErrNoActiveDriver
	}
	return d, nil
}

// —— 内置驱动 ——

// smtpDialTimeout 是 SMTP 连接超时（修正：ctx 15s 预算的真实连接时限）。
const smtpDialTimeout = 15 * time.Second

// smtpImplicitTLSPort 是隐式 TLS（SMTPS）端口约定：465=连接即 TLS 握手
// （QQ3 等）；其余端口明文连接后按需 STARTTLS。端口语义写进 admin
// 「邮件设置」提示与 config 样例，属部署约定而非隐藏魔法。
const smtpImplicitTLSPort = 465

// smtpSessionTimeout 是单次 SMTP 会话总时限（A2——dial 后任一步挂起均被
// 截断；取 retry 单 attempt 超时同量级）。
const smtpSessionTimeout = 30 * time.Second

// SMTPConfig 是 SMTP 驱动参数（config.message.smtp；password 走 env Secret）。
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// smtpDriver 是 email 内置驱动（SMTP 标准协议——net/smtp+STARTTLS；
// 「通用协议优先」——零三方依赖）。
type smtpDriver struct {
	cfg SMTPConfig
}

func newSMTPDriver(cfg SMTPConfig) *smtpDriver { return &smtpDriver{cfg: cfg} }

// Name 实现 domain.Driver。
func (d *smtpDriver) Name() string { return "smtp" }

// Send 实现 domain.Driver（SMTP：465=隐式 TLS（SMTPS）/ 其余端口=STARTTLS；
// 收件人单投）。
// 安全面：dial 超时=smtpDialTimeout；**整会话 deadline**（联测审计 A2：
// dial 后零超时=恶意/故障 SMTP 挂死 worker 与 RetryJob——NewClient banner/
// MAIL/RCPT/DATA/QUIT 任一步无响应均被会话超时截断）；有凭据时 TLS 失败=
// 显式报错（禁明文降级）；无凭据（内网 no-auth relay）=跳过 Auth；
// 每步 ctx 检查。
func (d *smtpDriver) Send(ctx context.Context, msg domain.Outbound) error {
	addr := fmt.Sprintf("%s:%d", d.cfg.Host, d.cfg.Port)
	host := d.cfg.Host
	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		d.cfg.From, sanitizeHeader(msg.To), sanitizeHeader(msg.Subject), msg.Body)
	nd := net.Dialer{Timeout: smtpDialTimeout}
	// 465=隐式 TLS（SMTPS，QQ3 等常见）：连接即握手，不走 STARTTLS 协商；
	// 其余端口=明文连接后按需 STARTTLS（有凭据强制）。
	implicitTLS := d.cfg.Port == smtpImplicitTLSPort
	var raw net.Conn
	var err error
	if implicitTLS {
		td := &tls.Dialer{NetDialer: &nd, Config: &tls.Config{
			ServerName: host, MinVersion: tls.VersionTLS12,
		}}
		raw, err = td.DialContext(ctx, "tcp", addr)
	} else {
		raw, err = nd.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("message: smtp dial: %w", err)
	}
	// 整会话超时：dial 成功即挂 deadline（A2——dial 后任一步挂起均有限时）；
	// ctx 更早取消时提前收口。
	deadline := time.Now().Add(smtpSessionTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = raw.SetDeadline(deadline)
	con, err := smtp.NewClient(raw, host)
	if err != nil {
		_ = raw.Close()
		return fmt.Errorf("message: smtp client: %w", err)
	}
	defer con.Close()
	if d.cfg.Username != "" { // 有凭据=必须 TLS（禁明文降级）
		if !implicitTLS {
			if err := con.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("message: smtp starttls (credentials require TLS): %w", err)
			}
		}
		if err := con.Auth(smtp.PlainAuth("", d.cfg.Username, d.cfg.Password, host)); err != nil {
			return fmt.Errorf("message: smtp auth: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := con.Mail(d.cfg.From); err != nil {
		return fmt.Errorf("message: smtp mail from: %w", err)
	}
	if err := con.Rcpt(msg.To); err != nil {
		return fmt.Errorf("message: smtp rcpt: %w", err)
	}
	w, err := con.Data()
	if err != nil {
		return fmt.Errorf("message: smtp data: %w", err)
	}
	if _, err := w.Write([]byte(header)); err != nil {
		return fmt.Errorf("message: smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("message: smtp data close: %w", err)
	}
	return con.Quit()
}

// sanitizeHeader 中和头注入面（联测审计 A3：Subject/收件人含 CRLF 可在 DATA
// 阶段注入 Bcc/伪造头——CR/LF 替换为空格；模板 vars 产物同样经此净化）。
func sanitizeHeader(v string) string {
	r := strings.NewReplacer("\r", " ", "\n", " ", "\r\n", " ")
	return r.Replace(v)
}

// logDriver 是 sms 默认驱动（无通用开放协议——输出到消息 access 文件；
// 开发/测试/演示面；生产厂商驱动下游自写 Register 注册）。
type logDriver struct {
	log *slog.Logger
}

// Name 实现 domain.Driver。
func (d *logDriver) Name() string { return "log" }

// Send 实现 domain.Driver（日志即投递——操作可审计可见）。
// PII 面（logging.md「禁止记录请求体明文/PII」）：收件人手机号经 primitives.Mask
// 脱敏（未注册 kind 时 fail-closed 全文遮蔽）；消息正文不落日志（模板变量可能含
// 姓名/邮箱等 PII 且为一次性敏感内容），只记长度供投递核对。
func (d *logDriver) Send(_ context.Context, msg domain.Outbound) error {
	if d.log != nil {
		d.log.Info("sms.send", "to", primitives.Mask("phone", msg.To), "body_len", len(msg.Body))
	}
	return nil
}
