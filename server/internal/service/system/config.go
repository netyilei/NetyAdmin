package system

import (
	"context"
	"log/slog"
	"strings"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemVO "NetyAdmin/internal/domain/vo/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/pubsub"
	systemRepo "NetyAdmin/internal/repository/system"
)

// publicConfigGroups 是允许公开查询的配置组白名单。
// 登录页仅需验证码开关等非敏感配置，email_config/sms_config 等含密钥的组必须鉴权访问。
var publicConfigGroups = map[string]bool{
	"cache_switches": true,
}

// sensitiveConfigKeys 是需要脱敏的配置键（小写匹配）。
// 即使鉴权后返回，这些字段也以 **** 脱敏，防止肩窥泄露。
var sensitiveConfigKeys = map[string]bool{
	"password":    true,
	"secret_id":   true,
	"secret_key":  true,
	"api_key":     true,
	"apikey":      true,
	"private_key": true,
}

type ConfigService interface {
	ListByGroup(ctx context.Context, groupName string) ([]*systemVO.SysConfigVO, error)
	ListByGroupPublic(ctx context.Context, groupName string) ([]*systemVO.SysConfigVO, error)
	Upsert(ctx context.Context, req *systemDto.UpdateConfigReq, operatorID uint) error
	BroadcastUpdate(ctx context.Context) error
	// TestEmail 测试邮件发送：使用当前邮件配置发送测试邮件，验证配置是否正确。
	// 收敛 Handler 跨层调用（B10）：Handler 不再直接调 emailDriver.Send。
	TestEmail(ctx context.Context, receiver string) error
}

type configService struct {
	repo        systemRepo.ConfigRepository
	watcher     configsync.ConfigWatcher
	eventBus    pubsub.EventBus
	emailDriver msgPkg.Driver
}

func NewConfigService(repo systemRepo.ConfigRepository, watcher configsync.ConfigWatcher, eventBus pubsub.EventBus, emailDriver msgPkg.Driver) ConfigService {
	return &configService{
		repo:        repo,
		watcher:     watcher,
		eventBus:    eventBus,
		emailDriver: emailDriver,
	}
}

func (s *configService) ListByGroup(ctx context.Context, groupName string) ([]*systemVO.SysConfigVO, error) {
	configs, err := s.repo.GetByGroup(ctx, groupName)
	if err != nil {
		return nil, err
	}

	items := make([]*systemVO.SysConfigVO, 0, len(configs))
	for _, c := range configs {
		val := c.ConfigValue
		if sensitiveConfigKeys[strings.ToLower(c.ConfigKey)] {
			val = "****"
		}
		items = append(items, &systemVO.SysConfigVO{
			GroupName:   c.GroupName,
			ConfigKey:   c.ConfigKey,
			ConfigValue: val,
			ValueType:   c.ValueType,
			Description: c.Description,
			IsSystem:    c.IsSystem,
		})
	}
	return items, nil
}

// ListByGroupPublic 仅供公开接口调用：仅允许白名单内的配置组，敏感字段脱敏。
func (s *configService) ListByGroupPublic(ctx context.Context, groupName string) ([]*systemVO.SysConfigVO, error) {
	if !publicConfigGroups[groupName] {
		return nil, errorx.New(errorx.CodeForbidden, "无权访问该配置组")
	}
	return s.ListByGroup(ctx, groupName)
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

	if err := s.watcher.ForceReload(ctx); err != nil {
		slog.Warn("force reload config failed", "err", err)
	}

	return s.BroadcastUpdate(ctx)
}

func (s *configService) BroadcastUpdate(ctx context.Context) error {
	if s.eventBus != nil {
		return s.eventBus.Publish(ctx, pubsub.TopicConfigSync, "config_updated")
	}
	return nil
}

// TestEmail 使用当前邮件配置发送测试邮件。
// 错误信息不含 err.Error()，原始错误用 slog 记录（B11 错误信息不泄露）。
func (s *configService) TestEmail(ctx context.Context, receiver string) error {
	if s.emailDriver == nil {
		return errorx.New(errorx.CodeInternalError, "邮件驱动未初始化")
	}
	err := s.emailDriver.Send(ctx, receiver, "NetyAdmin 测试邮件",
		"<h2>测试邮件</h2><p>这是一封来自 NetyAdmin 的测试邮件，如果您收到了此邮件，说明邮件配置正确。</p>", nil)
	if err != nil {
		slog.Error("test email send failed", "receiver", receiver, "err", err)
		return errorx.New(errorx.CodeEmailTestFailed)
	}
	return nil
}
