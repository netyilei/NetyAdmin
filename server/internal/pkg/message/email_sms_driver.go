package message

import (
	"context"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
)

type EmailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
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
	return cfg
}

func (d *emailDriver) Send(ctx context.Context, receiver string, title string, content string, params map[string]string) error {
	cfg := d.resolveConfig(ctx)
	if cfg.Host == "" || cfg.User == "" {
		return fmt.Errorf("email config incomplete: host or user is empty")
	}
	auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", receiver, title, content))

	err := smtp.SendMail(addr, auth, cfg.From, []string{receiver}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

type mockSmsDriver struct{}

func NewMockSmsDriver() SmsDriver {
	return &mockSmsDriver{}
}

func (d *mockSmsDriver) Send(ctx context.Context, receiver string, title string, content string, params map[string]string) error {
	fmt.Printf("[Mock SMS] Sending to %s: %s\n", receiver, content)
	return nil
}

func (d *mockSmsDriver) SendWithTemplate(ctx context.Context, phone string, templateID string, params map[string]string) error {
	paramStr := ""
	for k, v := range params {
		paramStr += fmt.Sprintf("%s=%s ", k, v)
	}
	fmt.Printf("[Mock SMS] Sending to %s using template %s, params: %s\n", phone, templateID, strings.TrimSpace(paramStr))
	return nil
}
