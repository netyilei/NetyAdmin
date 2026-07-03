package config

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server    ServerConfig    `toml:"server"`
	Database  DatabaseConfig  `toml:"database"`
	Redis     RedisConfig     `toml:"redis"`
	JWT       JWTConfig       `toml:"jwt"`
	Log       LogConfig       `toml:"log"`
	Migration MigrationConfig `toml:"migration"`
	Task      TaskConfig      `toml:"task"`
	Security  SecurityConfig  `toml:"security"`
	Email     EmailConfig     `toml:"email"`
	Sms       SmsConfig       `toml:"sms"`
	Bus       BusConfig       `toml:"bus"`
	Sentry    SentryConfig    `toml:"sentry"`
}

// SentryConfig Sentry 错误追踪配置（后端 Go）
// DSN 为空时自动禁用，不影响正常运行
type SentryConfig struct {
	DSN              string   `toml:"dsn"`                // Sentry DSN，为空则禁用
	Environment      string   `toml:"environment"`        // 环境标识（development / production）
	Release          string   `toml:"release"`            // 版本号
	SampleRate       *float64 `toml:"sample_rate"`        // 错误事件采样率 (0.0-1.0)；nil=未配置默认1.0，显式0=关闭错误上报
	TracesSampleRate float64  `toml:"traces_sample_rate"` // 性能追踪采样率 (0.0-1.0)；0=关闭性能追踪
}

type EmailConfig struct {
	Enabled        bool   `toml:"enabled"`
	Host           string `toml:"host"`
	Port           int    `toml:"port"`
	User           string `toml:"user"`
	Password       string `toml:"password"`
	From           string `toml:"from"`
	SSL            bool   `toml:"ssl"`
	StartTLS       bool   `toml:"starttls"`
	AuthType       string `toml:"auth_type"`
	ConnectTimeout int    `toml:"connect_timeout"`
	SendTimeout    int    `toml:"send_timeout"`
}

type SmsConfig struct {
	Enabled   bool   `toml:"enabled"`
	Driver    string `toml:"driver"`
	SecretID  string `toml:"secret_id"`
	SecretKey string `toml:"secret_key"`
	AppID     string `toml:"app_id"`
	SignName  string `toml:"sign_name"`
}

type SecurityConfig struct {
	AESKey string `toml:"aes_key"` // 系统加解密 Key (16, 24 或 32 字节)
}

type TaskConfig struct {
	Enabled bool                 `toml:"enabled"`
	Workers int                  `toml:"workers"`
	Jobs    map[string]JobConfig `toml:"jobs"`
}

type JobConfig struct {
	Enabled *bool   `toml:"enabled"` // 是否启用
	Type    *string `toml:"type"`    // 模式覆盖: once, cron, interval
	Spec    *string `toml:"spec"`    // 参数覆盖: 间隔时间或 Cron 表达式
	Weight  *int    `toml:"weight"`  // 权重覆盖 (0-100)
}

// MigrationConfig 数据库迁移配置。
// Dir 字段已移除：迁移文件以 go:embed 编译进二进制，不再从外部目录读取。
type MigrationConfig struct {
	Enabled bool `toml:"enabled"`
}

type ServerConfig struct {
	Port         int    `toml:"port"`
	Mode         string `toml:"mode"`
	ReadTimeout  int    `toml:"read_timeout"`
	WriteTimeout int    `toml:"write_timeout"`
	MultiNode    bool   `toml:"multi_node"` // 多机部署设为 true：会校验事件总线是否为 redis 模式
}

type DatabaseConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	DBName   string `toml:"dbname"`
	SSLMode  string `toml:"sslmode"`
	MaxIdle  int    `toml:"max_idle"`
	MaxOpen  int    `toml:"max_open"`
}

// DSN 返回 PostgreSQL 的 keyword=value 格式 DSN（pgx 通用）。
// 用于 GORM 连接（GORM postgres driver 使用 pgx 底层，接受 keyword=value 格式）。
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// DSNURL 返回 PostgreSQL 的 URL 格式 DSN（golang-migrate / lib/pq 通用）。
// golang-migrate 的 postgres driver 需要 URL 格式（带 postgres:// 协议头）。
func (c DatabaseConfig) DSNURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode)
}

type RedisConfig struct {
	Enabled  bool   `toml:"enabled"`
	Prefix   string `toml:"prefix"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Password string `toml:"password"`
	DB       int    `toml:"db"`

	L1Enabled       bool `toml:"l1_enabled"`
	LocalMaxSizeMB  int  `toml:"local_max_size_mb"`
	LocalMaxEntryKB int  `toml:"local_max_entry_kb"`
	LocalTTLMin     int  `toml:"local_ttl_min"`
}

type BusConfig struct {
	Driver string `toml:"driver"` // "redis" | "memory"，默认根据 Redis.Enabled 自动选择
}

type JWTConfig struct {
	Secret     string `toml:"secret"`
	Expiration int    `toml:"expiration"`
}

type LogConfig struct {
	Level      string `toml:"level"`
	Filename   string `toml:"filename"`
	MaxSize    int    `toml:"max_size"`
	MaxBackups int    `toml:"max_backups"`
	MaxAge     int    `toml:"max_age"`
	Compress   bool   `toml:"compress"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}
