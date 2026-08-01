package message

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	msgEntity "NetyAdmin/internal/domain/entity/message"
	msgDto "NetyAdmin/internal/interface/admin/dto/message"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/task"
	msgRepo "NetyAdmin/internal/repository/message"
)

var templateRe = regexp.MustCompile(`\{\{(.*?)\}\}`)

type MessageService interface {
	// SendTemplate 发送模板消息
	SendTemplate(ctx context.Context, code string, receiver string, params map[string]string) error
	// SendDirect 直接发送消息
	SendDirect(ctx context.Context, channel string, receiver string, title string, content string) error
	// ListTemplates 获取模板列表
	ListTemplates(ctx context.Context, req *msgDto.MsgTemplateQuery) ([]*msgEntity.MsgTemplate, int64, error)
	// ListRecords 获取记录列表
	ListRecords(ctx context.Context, req *msgDto.MsgRecordQuery) ([]*msgEntity.MsgRecord, int64, error)

	// Template Admin
	CreateTemplate(ctx context.Context, req *msgDto.CreateTemplateReq) error
	UpdateTemplate(ctx context.Context, id uint64, req *msgDto.UpdateTemplateReq) error
	DeleteTemplate(ctx context.Context, id uint64) error

	// Record Admin
	RetryRecord(ctx context.Context, id uint64) error

	// Client Internal Message
	ListUserInternalMsgs(ctx context.Context, userID string, page, pageSize int, readFilter *int) ([]*msgRepo.UserInternalMsg, int64, error)
	GetInternalMsgDetail(ctx context.Context, msgInternalID uint64, userID string) (*msgRepo.UserInternalMsg, error)
	MarkInternalMsgRead(ctx context.Context, msgInternalID uint64, userID string) error
	MarkAllInternalMsgRead(ctx context.Context, userID string) error
	CountUnreadInternalMsgs(ctx context.Context, userID string) (int64, error)
}

type messageService struct {
	repo       msgRepo.MsgRepository
	dispatcher task.Dispatcher
	drivers    map[string]msgPkg.Driver
	cacheFast  cache.ConfigCache
}

func NewMessageService(repo msgRepo.MsgRepository, dispatcher task.Dispatcher, drivers map[string]msgPkg.Driver, cacheFast cache.ConfigCache) MessageService {
	return &messageService{
		repo:       repo,
		dispatcher: dispatcher,
		drivers:    drivers,
		cacheFast:  cacheFast,
	}
}

func (s *messageService) SendTemplate(ctx context.Context, code string, receiver string, params map[string]string) error {
	var tpl msgEntity.MsgTemplate
	key := cache.KeyMsgTemplate(code)
	err := s.cacheFast.FetchFast(ctx, key, cache.TagMsgTemplate, []string{cache.TagMsgTemplate}, 3600*time.Second, &tpl, func() (interface{}, error) {
		return s.repo.GetTemplateByCode(ctx, code)
	})

	if err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	// 渲染模板内容
	content := s.renderTemplate(tpl.Content, params)

	// 创建发送记录
	rec := &msgEntity.MsgRecord{
		Channel:  tpl.Channel,
		Receiver: receiver,
		Title:    tpl.Title,
		Content:  content,
		Status:   msgEntity.MsgStatusPending, // 等待发送
		Priority: 1,                          // 模板消息通常优先级较高
	}

	if err := s.repo.CreateRecord(ctx, rec); err != nil {
		return err
	}

	// 投递异步任务 - 任务名为 msg_send_job，对应 MsgSendJob.Name()
	return s.dispatcher.Dispatch(ctx, "msg_send_job", rec.ID, task.WeightEssential)
}

func (s *messageService) SendDirect(ctx context.Context, channel string, receiver string, title string, content string) error {
	rec := &msgEntity.MsgRecord{
		Channel:  channel,
		Receiver: receiver,
		Title:    title,
		Content:  content,
		Status:   msgEntity.MsgStatusPending,
		Priority: 2,
	}

	if err := s.repo.CreateRecord(ctx, rec); err != nil {
		return err
	}

	// 投递异步任务 - 任务名为 msg_send_job
	return s.dispatcher.Dispatch(ctx, "msg_send_job", rec.ID, task.WeightNormal)
}

func (s *messageService) ListTemplates(ctx context.Context, req *msgDto.MsgTemplateQuery) ([]*msgEntity.MsgTemplate, int64, error) {
	// service 层接收 admin DTO，内部构造 repository query（spec B10：service 不应依赖 handler 构造的 repo 类型）
	repoQuery := &msgRepo.MsgRepoQuery{
		Page:     req.Current,
		PageSize: req.Size,
		Channel:  req.Channel,
		Code:     req.Code,
		Name:     req.Name,
		Status:   req.Status,
	}
	return s.repo.ListTemplates(ctx, repoQuery)
}

func (s *messageService) ListRecords(ctx context.Context, req *msgDto.MsgRecordQuery) ([]*msgEntity.MsgRecord, int64, error) {
	// service 层接收 admin DTO，内部构造 repository query（spec B10：service 不应依赖 handler 构造的 repo 类型）
	repoQuery := &msgRepo.MsgRepoQuery{
		Page:     req.Current,
		PageSize: req.Size,
		Channel:  req.Channel,
		Receiver: req.Receiver,
		Status:   req.Status,
	}
	return s.repo.ListRecords(ctx, repoQuery)
}

func (s *messageService) CreateTemplate(ctx context.Context, req *msgDto.CreateTemplateReq) error {
	// code 唯一性预校验
	existing, err := s.repo.GetTemplateByCode(ctx, req.Code)
	if err == nil && existing != nil {
		return errorx.New(errorx.CodeAlreadyExists, "模板编码已存在")
	}
	// 构造 entity
	tpl := &msgEntity.MsgTemplate{
		Code:          req.Code,
		Name:          req.Name,
		Channel:       req.Channel,
		Title:         req.Title,
		Content:       req.Content,
		ProviderTplID: req.ProviderTplID,
		Status:        req.Status,
	}
	if tpl.Status == 0 {
		tpl.Status = msgEntity.MsgTplStatusEnabled // 默认启用
	}
	if err := s.repo.CreateTemplate(ctx, tpl); err != nil {
		return err
	}
	// 失效缓存（与 Update/Delete 风格统一）
	if err := s.cacheFast.InvalidateByTags(ctx, cache.TagMsgTemplate); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagMsgTemplate, "err", err)
	}
	return nil
}

func (s *messageService) UpdateTemplate(ctx context.Context, id uint64, req *msgDto.UpdateTemplateReq) error {
	// 先查旧记录（保留 ID/CreatedAt/DeletedAt 不被覆盖）
	existing, err := s.repo.GetTemplateByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "消息模板不存在")
		}
		slog.Error("repo.GetTemplateByID failed", "templateID", id, "err", err)
		return fmt.Errorf("repo.GetTemplateByID: %w", err)
	}
	// patch 旧 entity（保留 ID/CreatedAt/DeletedAt，Code 创建后不可变更）
	existing.Name = req.Name
	existing.Channel = req.Channel
	existing.Title = req.Title
	existing.Content = req.Content
	existing.ProviderTplID = req.ProviderTplID
	existing.Status = req.Status
	if err := s.repo.UpdateTemplate(ctx, existing); err != nil {
		return err
	}
	if err := s.cacheFast.InvalidateByTags(ctx, cache.TagMsgTemplate); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagMsgTemplate, "err", err)
	}
	return nil
}

func (s *messageService) DeleteTemplate(ctx context.Context, id uint64) error {
	if err := s.repo.DeleteTemplate(ctx, id); err != nil {
		return err
	}
	// 失效缓存
	return s.cacheFast.InvalidateByTags(ctx, cache.TagMsgTemplate)
}

func (s *messageService) RetryRecord(ctx context.Context, id uint64) error {
	rec, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return err
	}

	if rec.Status != msgEntity.MsgStatusFailed {
		return fmt.Errorf("only failed records can be retried")
	}

	// 重置状态为等待发送
	rec.Status = msgEntity.MsgStatusPending
	rec.ErrorMsg = ""
	if err := s.repo.UpdateRecord(ctx, rec); err != nil {
		return err
	}

	// 重新投递任务
	return s.dispatcher.Dispatch(ctx, "msg_send_job", rec.ID, task.WeightEssential)
}

func (s *messageService) renderTemplate(content string, params map[string]string) string {
	return templateRe.ReplaceAllStringFunc(content, func(match string) string {
		key := strings.Trim(match, "{} ")
		if val, ok := params[key]; ok {
			return val
		}
		return match
	})
}

func (s *messageService) ListUserInternalMsgs(ctx context.Context, userID string, page, pageSize int, readFilter *int) ([]*msgRepo.UserInternalMsg, int64, error) {
	return s.repo.ListUserInternalMsgs(ctx, userID, page, pageSize, readFilter)
}

func (s *messageService) GetInternalMsgDetail(ctx context.Context, msgInternalID uint64, userID string) (*msgRepo.UserInternalMsg, error) {
	msg, err := s.repo.GetInternalMsgDetail(ctx, msgInternalID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeMsgRecordNotFound)
		}
		return nil, err
	}
	return msg, nil
}

func (s *messageService) MarkInternalMsgRead(ctx context.Context, msgInternalID uint64, userID string) error {
	return s.repo.MarkInternalMsgRead(ctx, msgInternalID, userID)
}

func (s *messageService) MarkAllInternalMsgRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllInternalMsgRead(ctx, userID)
}

func (s *messageService) CountUnreadInternalMsgs(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountUnreadInternalMsgs(ctx, userID)
}
