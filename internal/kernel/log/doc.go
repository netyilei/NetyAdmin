// Package log 提供运行日志：log/slog + lumberjack + Manager（不引入 logrus/自研框架）。
// 运行/错误日志写 stdout/文件/采集器，禁止写业务 DB；路径 logs/<YYYY-MM-DD>/<module>_<type>.log；
// 类型 app/access/error/security/audit；全局 level + 模块/类型开关动态生效；PII/密钥脱敏。
// 规范见 docs/rules/logging.md。
package log
