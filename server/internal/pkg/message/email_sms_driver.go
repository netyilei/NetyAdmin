package message

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

type EmailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

type emailDriver struct {
	cfg EmailConfig
}

func NewEmailDriver(cfg EmailConfig) Driver {
	return &emailDriver{cfg: cfg}
}

func (d *emailDriver) Send(ctx context.Context, receiver string, title string, content string, params map[string]string) error {
	auth := smtp.PlainAuth("", d.cfg.User, d.cfg.Password, d.cfg.Host)
	addr := fmt.Sprintf("%s:%d", d.cfg.Host, d.cfg.Port)

	// 构造简单的邮件报文
	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", receiver, title, content))

	err := smtp.SendMail(addr, auth, d.cfg.From, []string{receiver}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// --- Mock SMS Driver ---

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
