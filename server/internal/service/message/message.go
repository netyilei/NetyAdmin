package message

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	msgEntity "NetyAdmin/internal/domain/entity/message"
	"NetyAdmin/internal/pkg/cache"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/task"
	msgRepo "NetyAdmin/internal/repository/message"
)

type MessageService interface {
	// SendTemplate 发送模板消息
	SendTemplate(ctx context.Context, code string, receiver string, params map[string]string) error
	// SendDirect 直接发送消息
	SendDirect(ctx context.Context, channel string, receiver string, title string, content string) error
	// ListTemplates 获取模板列表
	ListTemplates(ctx context.Context, query *msgRepo.MsgRepoQuery) ([]*msgEntity.MsgTemplate, int64, error)
	// ListRecords 获取记录列表
	ListRecords(ctx context.Context, query *msgRepo.MsgRepoQuery) ([]*msgEntity.MsgRecord, int64, error)

	// Template Admin
	CreateTemplate(ctx context.Context, tpl *msgEntity.MsgTemplate) error
	UpdateTemplate(ctx context.Context, tpl *msgEntity.MsgTemplate) error
	DeleteTemplate(ctx context.Context, id uint64) error

	// Record Admin
	RetryRecord(ctx context.Context, id uint64) error
}

type messageService struct {
	repo       msgRepo.MsgRepository
	dispatcher task.Dispatcher
	drivers    map[string]msgPkg.Driver
	cacheMgr   cache.LazyCacheManager
}

func NewMessageService(repo msgRepo.MsgRepository, dispatcher task.Dispatcher, drivers map[string]msgPkg.Driver, cacheMgr cache.LazyCacheManager) MessageService {
	return &messageService{
		repo:       repo,
		dispatcher: dispatcher,
		drivers:    drivers,
		cacheMgr:   cacheMgr,
	}
}

func (s *messageService) SendTemplate(ctx context.Context, code string, receiver string, params map[string]string) error {
	var tpl msgEntity.MsgTemplate
	key := cache.KeyMsgTemplate(code)
	err := s.cacheMgr.Fetch(ctx, key, cache.TagMsgTemplate, []string{cache.TagMsgTemplate}, 3600*time.Second, &tpl, func() (interface{}, error) {
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

func (s *messageService) ListTemplates(ctx context.Context, query *msgRepo.MsgRepoQuery) ([]*msgEntity.MsgTemplate, int64, error) {
	return s.repo.ListTemplates(ctx, query)
}

func (s *messageService) ListRecords(ctx context.Context, query *msgRepo.MsgRepoQuery) ([]*msgEntity.MsgRecord, int64, error) {
	return s.repo.ListRecords(ctx, query)
}

func (s *messageService) CreateTemplate(ctx context.Context, tpl *msgEntity.MsgTemplate) error {
	return s.repo.CreateTemplate(ctx, tpl)
}

func (s *messageService) UpdateTemplate(ctx context.Context, tpl *msgEntity.MsgTemplate) error {
	if err := s.repo.UpdateTemplate(ctx, tpl); err != nil {
		return err
	}
	// 失效缓存
	return s.cacheMgr.InvalidateByTags(ctx, cache.TagMsgTemplate)
}

func (s *messageService) DeleteTemplate(ctx context.Context, id uint64) error {
	if err := s.repo.DeleteTemplate(ctx, id); err != nil {
		return err
	}
	// 失效缓存
	return s.cacheMgr.InvalidateByTags(ctx, cache.TagMsgTemplate)
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
	re := regexp.MustCompile(`\{\{(.*?)\}\}`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		key := strings.Trim(match, "{} ")
		if val, ok := params[key]; ok {
			return val
		}
		return match
	})
}
