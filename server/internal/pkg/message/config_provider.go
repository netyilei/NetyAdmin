package message

import (
	"context"

	systemRepo "NetyAdmin/internal/repository/system"
)

type dbConfigProvider struct {
	repo systemRepo.ConfigRepository
}

func NewDbConfigProvider(repo systemRepo.ConfigRepository) ConfigProvider {
	return &dbConfigProvider{repo: repo}
}

func (p *dbConfigProvider) GetByGroup(ctx context.Context, groupName string) (map[string]string, error) {
	configs, err := p.repo.GetByGroup(ctx, groupName)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(configs))
	for _, c := range configs {
		result[c.ConfigKey] = c.ConfigValue
	}
	return result, nil
}
