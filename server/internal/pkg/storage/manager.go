package storage

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
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

// GenerateObjectKey 生成不带业务前缀的对象 key。
// 等价于 GenerateObjectKeyWithBusiness(originalName, pathPrefix, "", "")，
// 作为常用场景的便捷语法糖（重构清单 B-OTHER-3）。
func GenerateObjectKey(originalName string, pathPrefix string) string {
	return GenerateObjectKeyWithBusiness(originalName, pathPrefix, "", "")
}

// GenerateObjectKeyWithBusiness 生成对象 key 的统一实现。
//
// key 结构：[pathPrefix/][businessType/businessID/]date/hash.ext
//   - businessType 非空时插入业务前缀路径，便于按业务隔离与统计
//   - pathPrefix 非空时在最外层加路径前缀（桶内子目录）
//
// 原实现 GenerateObjectKey 与本函数 90% 重复（hash/datePath/ext 计算完全一致），
// 现统一为本函数单一实现，GenerateObjectKey 委托调用。
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

	ext := strings.ToLower(GetFileExtension(filename))
	if ext == "" {
		return false
	}

	allowedExts := splitAndTrim(allowedTypes, ",")
	for _, allowed := range allowedExts {
		a := strings.ToLower(allowed)
		if ext == a || "."+ext == a {
			return true
		}
	}
	return false
}

func splitAndTrim(s string, sep string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}
