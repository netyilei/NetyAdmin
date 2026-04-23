package message

import (
	"context"

	"NetyAdmin/internal/pkg/configsync"
)

type watcherConfigProvider struct {
	watcher configsync.ConfigWatcher
}

func NewWatcherConfigProvider(watcher configsync.ConfigWatcher) ConfigProvider {
	return &watcherConfigProvider{watcher: watcher}
}

func (p *watcherConfigProvider) GetByGroup(ctx context.Context, groupName string) (map[string]string, error) {
	return p.watcher.GetGroupConfigs(groupName), nil
}
