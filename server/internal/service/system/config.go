package system

import (
	"context"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemVO "NetyAdmin/internal/domain/vo/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/pubsub"
	systemRepo "NetyAdmin/internal/repository/system"
)

type ConfigService interface {
	ListByGroup(ctx context.Context, groupName string) ([]*systemVO.SysConfigVO, error)
	Upsert(ctx context.Context, req *systemDto.UpdateConfigReq, operatorID uint) error
	BroadcastUpdate(ctx context.Context) error
}

type configService struct {
	repo     systemRepo.ConfigRepository
	watcher  configsync.ConfigWatcher
	eventBus pubsub.EventBus
}

func NewConfigService(repo systemRepo.ConfigRepository, watcher configsync.ConfigWatcher, eventBus pubsub.EventBus) ConfigService {
	return &configService{
		repo:     repo,
		watcher:  watcher,
		eventBus: eventBus,
	}
}

func (s *configService) ListByGroup(ctx context.Context, groupName string) ([]*systemVO.SysConfigVO, error) {
	configs, err := s.repo.GetByGroup(ctx, groupName)
	if err != nil {
		return nil, err
	}

	items := make([]*systemVO.SysConfigVO, 0, len(configs))
	for _, c := range configs {
		items = append(items, &systemVO.SysConfigVO{
			GroupName:   c.GroupName,
			ConfigKey:   c.ConfigKey,
			ConfigValue: c.ConfigValue,
			ValueType:   c.ValueType,
			Description: c.Description,
			IsSystem:    c.IsSystem,
		})
	}
	return items, nil
}

func (s *configService) Upsert(ctx context.Context, req *systemDto.UpdateConfigReq, operatorID uint) error {
	configItem := &systemEntity.SysConfig{
		GroupName:   req.GroupName,
		ConfigKey:   req.ConfigKey,
		ConfigValue: req.ConfigValue,
		ValueType:   req.ValueType,
		Description: req.Description,
	}
	configItem.UpdatedBy = operatorID

	if err := s.repo.Upsert(ctx, configItem); err != nil {
		return err
	}

	_ = s.watcher.ForceReload(ctx)

	return s.BroadcastUpdate(ctx)
}

func (s *configService) BroadcastUpdate(ctx context.Context) error {
	if s.eventBus != nil {
		return s.eventBus.Publish(ctx, pubsub.TopicConfigSync, "config_updated")
	}
	return nil
}
