package message

import (
	"context"
	"fmt"
	"strconv"
	"time"

	mail "github.com/xhit/go-simple-mail/v2"
)

type EmailConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	From           string
	SSL            bool
	StartTLS       bool
	AuthType       string
	ConnectTimeout int
	SendTimeout    int
}

type ConfigProvider interface {
	GetByGroup(ctx context.Context, groupName string) (map[string]string, error)
}

type emailDriver struct {
	cfg      EmailConfig
	provider ConfigProvider
}

func NewEmailDriver(cfg EmailConfig, provider ConfigProvider) Driver {
	return &emailDriver{cfg: cfg, provider: provider}
}

func (d *emailDriver) resolveConfig(ctx context.Context) EmailConfig {
	cfg := d.cfg
	if d.provider == nil {
		return cfg
	}
	vals, err := d.provider.GetByGroup(ctx, "email_config")
	if err != nil || len(vals) == 0 {
		return cfg
	}
	if v, ok := vals["host"]; ok && v != "" {
		cfg.Host = v
	}
	if v, ok := vals["port"]; ok && v != "" {
		if p, e := strconv.Atoi(v); e == nil && p > 0 {
			cfg.Port = p
		}
	}
	if v, ok := vals["user"]; ok && v != "" {
		cfg.User = v
	}
	if v, ok := vals["password"]; ok && v != "" {
		cfg.Password = v
	}
	if v, ok := vals["from"]; ok && v != "" {
		cfg.From = v
	}
	if v, ok := vals["ssl_enabled"]; ok {
		cfg.SSL = v == "true"
	}
	if v, ok := vals["starttls_enabled"]; ok {
		cfg.StartTLS = v == "true"
	}
	if v, ok := vals["auth_type"]; ok && v != "" {
		cfg.AuthType = v
	}
	if v, ok := vals["connect_timeout"]; ok && v != "" {
		if t, e := strconv.Atoi(v); e == nil && t > 0 {
			cfg.ConnectTimeout = t
		}
	}
	if v, ok := vals["send_timeout"]; ok && v != "" {
		if t, e := strconv.Atoi(v); e == nil && t > 0 {
			cfg.SendTimeout = t
		}
	}
	return cfg
}

func (d *emailDriver) buildSMTPServer(cfg EmailConfig) *mail.SMTPServer {
	server := mail.NewSMTPClient()
	server.Host = cfg.Host
	server.Port = cfg.Port
	server.Username = cfg.User
	server.Password = cfg.Password
	server.Encryption = d.resolveEncryption(cfg)
	server.Authentication = d.resolveAuthMode(cfg)

	if cfg.ConnectTimeout > 0 {
		server.ConnectTimeout = time.Duration(cfg.ConnectTimeout) * time.Second
	} else {
		server.ConnectTimeout = 30 * time.Second
	}
	if cfg.SendTimeout > 0 {
		server.SendTimeout = time.Duration(cfg.SendTimeout) * time.Second
	} else {
		server.SendTimeout = 30 * time.Second
	}

	server.KeepAlive = false

	return server
}

func (d *emailDriver) resolveEncryption(cfg EmailConfig) mail.Encryption {
	if cfg.SSL {
		return mail.EncryptionSSLTLS
	}
	if cfg.StartTLS {
		return mail.EncryptionSTARTTLS
	}
	return mail.EncryptionNone
}

func (d *emailDriver) resolveAuthMode(cfg EmailConfig) mail.AuthType {
	switch cfg.AuthType {
	case "plain":
		return mail.AuthPlain
	case "login":
		return mail.AuthLogin
	case "crammd5":
		return mail.AuthCRAMMD5
	case "auto":
		return mail.AuthAuto
	default:
		return mail.AuthPlain
	}
}

func (d *emailDriver) Send(ctx context.Context, receiver string, title string, content string, params map[string]string) error {
	cfg := d.resolveConfig(ctx)
	if cfg.Host == "" || cfg.User == "" {
		return fmt.Errorf("email config incomplete: host or user is empty")
	}

	smtpServer := d.buildSMTPServer(cfg)
	client, err := smtpServer.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server %s:%d: %w", cfg.Host, cfg.Port, err)
	}
	defer client.Close()

	email := mail.NewMSG()
	email.SetFrom(cfg.From).
		AddTo(receiver).
		SetSubject(title)

	email.SetBody(mail.TextHTML, content)

	if email.Error != nil {
		return fmt.Errorf("failed to build email message: %w", email.Error)
	}

	if err := email.Send(client); err != nil {
		return fmt.Errorf("failed to send email to %s: %w", receiver, err)
	}

	return nil
}

type mockSmsDriver struct{}

func NewMockSmsDriver() SmsDriver {
	return &mockSmsDriver{}
}

func (d *mockSmsDriver) Send(ctx context.Context, receiver string, title string, content string, params map[string]string) error {
	return nil
}

func (d *mockSmsDriver) SendWithTemplate(ctx context.Context, phone string, templateID string, params map[string]string) error {
	return nil
}
