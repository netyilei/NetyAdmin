package configsync

import (
	"context"
	"log"
	"strings"
	"sync"

	systemRepo "NetyAdmin/internal/repository/system"
)

type ConfigWatcher interface {
	IsCacheEnabled(moduleName string) bool
	GetConfig(groupName, key string) (string, bool)
	GetGroupConfigs(groupName string) map[string]string
	ForceReload(ctx context.Context) error
}

type configWatcher struct {
	repo systemRepo.ConfigRepository

	sync.RWMutex
	memory map[string]string
}

func NewConfigWatcher(repo systemRepo.ConfigRepository) ConfigWatcher {
	w := &configWatcher{
		repo:   repo,
		memory: make(map[string]string),
	}

	if err := w.ForceReload(context.Background()); err != nil {
		log.Printf("[ConfigWatcher] 首次加载全局配置失败: %v", err)
	}
	return w
}

func (w *configWatcher) buildMemKey(group, key string) string {
	return group + ":" + key
}

func (w *configWatcher) ForceReload(ctx context.Context) error {
	configs, err := w.repo.GetAll(ctx)
	if err != nil {
		return err
	}

	newMem := make(map[string]string, len(configs))
	for _, config := range configs {
		newMem[w.buildMemKey(config.GroupName, config.ConfigKey)] = config.ConfigValue
	}

	w.Lock()
	w.memory = newMem
	w.Unlock()

	log.Printf("[Config Watcher] 成功从 DB 同步 %d 条配置记录", len(configs))
	return nil
}

func (w *configWatcher) GetConfig(groupName, key string) (string, bool) {
	w.RLock()
	defer w.RUnlock()

	val, exists := w.memory[w.buildMemKey(groupName, key)]
	return val, exists
}

func (w *configWatcher) GetGroupConfigs(groupName string) map[string]string {
	w.RLock()
	defer w.RUnlock()

	result := make(map[string]string)
	prefix := groupName + ":"
	for k, v := range w.memory {
		if strings.HasPrefix(k, prefix) {
			key := strings.TrimPrefix(k, prefix)
			result[key] = v
		}
	}
	return result
}

// IsCacheEnabled 实现 SwitchChecker 接口
func (w *configWatcher) IsCacheEnabled(moduleName string) bool {
	val, exists := w.GetConfig("cache_switches", moduleName)
	if !exists {
		// 默认保险策略：查不到配置默认允许缓存（以保持旧逻辑兼容性）
		return true
	}
	return strings.ToLower(val) == "true" || val == "1"
}
