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
