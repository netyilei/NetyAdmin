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
}

type EmailConfig struct {
	Enabled  bool   `toml:"enabled"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	From     string `toml:"from"`
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
	Jobs    map[string]JobConfig `toml:"jobs"`
}

type JobConfig struct {
	Enabled *bool   `toml:"enabled"` // 是否启用
	Type    *string `toml:"type"`    // 模式覆盖: once, cron, interval
	Spec    *string `toml:"spec"`    // 参数覆盖: 间隔时间或 Cron 表达式
	Weight  *int    `toml:"weight"`  // 权重覆盖 (0-100)
}

type MigrationConfig struct {
	Enabled bool   `toml:"enabled"`
	Dir     string `toml:"dir"`
}

type ServerConfig struct {
	Port         int    `toml:"port"`
	Mode         string `toml:"mode"`
	ReadTimeout  int    `toml:"read_timeout"`
	WriteTimeout int    `toml:"write_timeout"`
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

type RedisConfig struct {
	Enabled  bool   `toml:"enabled"`
	Prefix   string `toml:"prefix"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Password string `toml:"password"`
	DB       int    `toml:"db"`

	// L1 缓存配置
	L1Enabled       bool `toml:"l1_enabled"`
	LocalMaxSizeMB  int  `toml:"local_max_size_mb"`  // 最大本地缓存大小 (MB)
	LocalMaxEntryKB int  `toml:"local_max_entry_kb"` // 单条记录最大大小 (KB)
	LocalTTLMin     int  `toml:"local_ttl_min"`      // 本地缓存过期时间 (分钟)
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
