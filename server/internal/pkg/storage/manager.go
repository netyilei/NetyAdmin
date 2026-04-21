package storage

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"
)

type Manager struct {
	drivers map[uint]Driver
	configs map[uint]*Config
	mu      sync.RWMutex
	factory DriverFactory
}

func NewManager(factory DriverFactory) *Manager {
	return &Manager{
		drivers: make(map[uint]Driver),
		configs: make(map[uint]*Config),
		factory: factory,
	}
}

func (m *Manager) Register(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	driver, err := m.factory.Create(config)
	if err != nil {
		return fmt.Errorf("failed to create driver for config %d: %w", config.ID, err)
	}

	m.drivers[config.ID] = driver
	m.configs[config.ID] = config
	return nil
}

func (m *Manager) Unregister(configID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.drivers, configID)
	delete(m.configs, configID)
}

func (m *Manager) GetDriver(configID uint) (Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	driver, ok := m.drivers[configID]
	if !ok {
		return nil, fmt.Errorf("storage config %d not found", configID)
	}
	return driver, nil
}

func (m *Manager) GetConfig(configID uint) (*Config, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, ok := m.configs[configID]
	if !ok {
		return nil, fmt.Errorf("storage config %d not found", configID)
	}
	return config, nil
}

func (m *Manager) GetDefaultDriver() (Driver, *Config, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, config := range m.configs {
		if config.IsDefault && config.IsEnabled() {
			return m.drivers[id], config, nil
		}
	}

	for id, config := range m.configs {
		if config.IsEnabled() {
			return m.drivers[id], config, nil
		}
	}

	return nil, nil, fmt.Errorf("no available storage config")
}

func (m *Manager) Upload(ctx context.Context, configID uint, key string, reader io.Reader, size int64, contentType string) (*UploadResult, error) {
	driver, err := m.GetDriver(configID)
	if err != nil {
		return nil, err
	}
	return driver.Upload(ctx, key, reader, size, contentType)
}

func (m *Manager) Delete(ctx context.Context, configID uint, key string) error {
	driver, err := m.GetDriver(configID)
	if err != nil {
		return err
	}
	return driver.Delete(ctx, key)
}

func (m *Manager) GetPresignedUploadURL(ctx context.Context, configID uint, key string, contentType string, expires time.Duration) (string, error) {
	driver, err := m.GetDriver(configID)
	if err != nil {
		return "", err
	}
	return driver.GetPresignedUploadURL(ctx, key, contentType, expires)
}

func (m *Manager) GetPresignedDownloadURL(ctx context.Context, configID uint, key string, expires time.Duration) (string, error) {
	driver, err := m.GetDriver(configID)
	if err != nil {
		return "", err
	}
	return driver.GetPresignedDownloadURL(ctx, key, expires)
}

func GenerateObjectKey(originalName string, pathPrefix string) string {
	ext := filepath.Ext(originalName)
	timestamp := time.Now().UnixNano()
	hash := md5.Sum([]byte(fmt.Sprintf("%s%d", originalName, timestamp)))
	hashStr := hex.EncodeToString(hash[:])[:16]

	datePath := time.Now().Format("2006/01/02")

	key := fmt.Sprintf("%s/%s%s", datePath, hashStr, ext)
	if pathPrefix != "" {
		key = fmt.Sprintf("%s/%s", pathPrefix, key)
	}
	return key
}

func GenerateObjectKeyWithBusiness(originalName string, pathPrefix string, businessType string, businessID string) string {
	ext := filepath.Ext(originalName)
	timestamp := time.Now().UnixNano()
	hash := md5.Sum([]byte(fmt.Sprintf("%s%d", originalName, timestamp)))
	hashStr := hex.EncodeToString(hash[:])[:16]

	datePath := time.Now().Format("2006/01/02")

	var key string
	if businessType != "" {
		key = fmt.Sprintf("%s/%s/%s/%s%s", businessType, businessID, datePath, hashStr, ext)
	} else {
		key = fmt.Sprintf("%s/%s%s", datePath, hashStr, ext)
	}

	if pathPrefix != "" {
		key = fmt.Sprintf("%s/%s", pathPrefix, key)
	}
	return key
}

func FormatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func GetFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if len(ext) > 0 && ext[0] == '.' {
		return ext[1:]
	}
	return ext
}

func IsAllowedFileType(filename string, allowedTypes string) bool {
	if allowedTypes == "" {
		return true
	}

	ext := GetFileExtension(filename)
	if ext == "" {
		return false
	}

	allowedExts := splitAndTrim(allowedTypes, ",")
	for _, allowed := range allowedExts {
		if ext == allowed || "."+ext == allowed {
			return true
		}
	}
	return false
}

func splitAndTrim(s string, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || string(s[i]) == sep {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	return result
}
