package sentry

import (
	"fmt"
	"log/slog"
	"strconv"
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
	sampleRate := cfg.SampleRate
	if sampleRate == 0 {
		sampleRate = 1.0
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
	}

	if err := sentry.Init(opts); err != nil {
		return fmt.Errorf("Sentry 初始化失败: %w", err)
	}

	slog.Info("Sentry 初始化成功",
		"environment", env,
		"release", cfg.Release,
		"sample_rate", sampleRate,
		"traces_sample_rate", cfg.TracesSampleRate,
	)
	return nil
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
