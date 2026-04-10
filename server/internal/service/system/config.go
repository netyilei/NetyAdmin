package system

import (
	"context"

	"NetyAdmin/internal/config"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemVO "NetyAdmin/internal/domain/vo/system"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/redis"
	systemRepo "NetyAdmin/internal/repository/system"

	goRedis "github.com/redis/go-redis/v9"
)

type ConfigService interface {
	ListByGroup(ctx context.Context, groupName string) ([]*systemVO.SysConfigVO, error)
	Upsert(ctx context.Context, req *systemDto.UpdateConfigReq, operatorID uint) error
	BroadcastUpdate(ctx context.Context) error
}

type configService struct {
	repo        systemRepo.ConfigRepository
	redisClient *goRedis.Client
	redisCfg    *config.RedisConfig
	watcher     configsync.ConfigWatcher
}

func NewConfigService(repo systemRepo.ConfigRepository, redisClient *goRedis.Client, redisCfg *config.RedisConfig, watcher configsync.ConfigWatcher) ConfigService {
	return &configService{
		repo:        repo,
		redisClient: redisClient,
		redisCfg:    redisCfg,
		watcher:     watcher,
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

	// 更新成功后广播并强制自身热重载
	// 针对无需 Redis 的本机环境，我们起码强制让当前的 Watcher 重载内存
	_ = s.watcher.ForceReload(ctx)

	// 如果 Redis Client 存在，触发全网广播告知其他节点更新内存
	return s.BroadcastUpdate(ctx)
}

func (s *configService) BroadcastUpdate(ctx context.Context) error {
	if s.redisClient != nil && s.redisCfg != nil {
		channel := redis.ChannelConfigSync(s.redisCfg.Prefix)
		return s.redisClient.Publish(ctx, channel, "config_updated").Err()
	}
	return nil
}
