// Package config 提供分层配置：默认值 → 文件 → 环境变量 → 动态配置（configcenter）→ Secrets。
// 模块配置段绑定（Manifest config_section + schema）、启动期校验、未知段告警。
// 监听地址/DSN/Redis 地址/模块启停不热更（docs/rules/hot-update.md）。
package config
