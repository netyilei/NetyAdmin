// Package slogutil provides a slog.Logger helper that automatically attaches
// the request_id field from context.
//
// 使用范式（service / repository / async worker 入口）：
//
//	logger := slogutil.LoggerFromContext(ctx)
//	logger.Error("xxx failed", "err", err)
//	logger.Info("cache invalidated", "tags", tags)
//
// 不强制改造全项目所有 slog 调用——仅在关键入口（logbus 刷盘、task 执行、cache 失效）
// 引入此 helper 作为示范，其余调用点可渐进迁移。
package slogutil

import (
	"context"
	"log/slog"

	"NetyAdmin/internal/pkg/requestid"
)

// LoggerFromContext 返回一个带 request_id 字段的 *slog.Logger。
// ctx 中无 request_id 时返回 slog.Default()（保持原有日志行为，不丢日志）。
//
// 设计意图：让 service / repository / async worker 通过 ctx 自动携带 request_id，
// 无需在每个 slog 调用点重复 slog.With("request_id", ...) 模板代码。
func LoggerFromContext(ctx context.Context) *slog.Logger {
	rid := requestid.FromContext(ctx)
	if rid == "" {
		return slog.Default()
	}
	return slog.Default().With(slog.String(requestid.MetaKey, rid))
}
