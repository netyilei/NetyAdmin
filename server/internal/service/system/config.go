package system

import (
	"context"
	"errors"
	"log/slog"

	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemVO "NetyAdmin/internal/domain/vo/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/mask"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/pubsub"
	systemRepo "NetyAdmin/internal/repository/system"
)

// publicConfigGroups 是允许未认证公开查询的配置组白名单。
// 仅登录页等前置场景所需的非敏感配置组可加入此白名单；
// email_config / sms_config 等含密钥的组必须通过鉴权接口访问。
var publicConfigGroups = map[string]bool{
	// 登录页需读取 admin_login_enabled 判断是否展示验证码
	"captcha_config": true,
}

type ConfigService interface {
	// ListByGroup 返回指定分组的配置（敏感字段脱敏）。
	// 供已鉴权的管理后台接口调用，允许访问任意分组。
	ListByGroup(ctx context.Context, groupName string) ([]*systemVO.SysConfigVO, error)
	// ListByGroupPublic 返回指定分组的配置（敏感字段脱敏）。
	// 仅供公开接口调用：仅允许白名单内的分组，其余返回 CodeForbidden。
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

// ListByGroup 返回指定分组的配置列表，敏感字段值替换为 mask.MaskPlaceholder。
// 引用 mask.IsSensitive 做归一化匹配（RULES.md §11.4），禁止本地硬编码字段列表。
func (s *configService) ListByGroup(ctx context.Context, groupName string) ([]*systemVO.SysConfigVO, error) {
	configs, err := s.repo.GetByGroup(ctx, groupName)
	if err != nil {
		return nil, err
	}

	items := make([]*systemVO.SysConfigVO, 0, len(configs))
	for _, c := range configs {
		val := c.ConfigValue
		if mask.IsSensitive(c.ConfigKey) {
			val = mask.MaskPlaceholder
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

// Upsert 新增或更新单个配置项。
//
// 敏感字段占位保护：前端 GET 配置时看到 mask.MaskPlaceholder（****），
// 若用户未修改该字段直接 PUT 回来，req.ConfigValue 会等于 MaskPlaceholder。
// 此时必须保留 DB 旧值，否则会用字面量 **** 覆盖真实密码/密钥。
//
// 实现采用 fetch+patch+Save 模式（SHARED.md §二）：
//  1. 先按 (groupName, configKey) 查询旧记录
//  2. 若存在且该 key 为敏感字段且新值 == MaskPlaceholder，保留旧 ConfigValue
//  3. 调 repo.Upsert 写入
func (s *configService) Upsert(ctx context.Context, req *systemDto.UpdateConfigReq, operatorID uint) error {
	valueToWrite := req.ConfigValue

	// 敏感字段占位保护：查旧记录，若新值是脱敏占位则保留旧值
	if mask.IsSensitive(req.ConfigKey) && req.ConfigValue == mask.MaskPlaceholder {
		old, err := s.repo.GetByGroupAndKey(ctx, req.GroupName, req.ConfigKey)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			// 旧记录不存在（首次创建）：不允许写入占位符作为真实值
			slog.Warn("config: refusing to create sensitive config with mask placeholder",
				"group", req.GroupName, "key", req.ConfigKey)
			return errorx.New(errorx.CodeInvalidParams, "敏感配置不允许使用占位值")
		}
		valueToWrite = old.ConfigValue
	}

	configItem := &systemEntity.SysConfig{
		GroupName:   req.GroupName,
		ConfigKey:   req.ConfigKey,
		ConfigValue: valueToWrite,
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
