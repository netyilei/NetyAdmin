// service.go：消息服务——模板 CRUD/发送/重发/记录/重试 Job。
// 发送路径：模板渲染 → active 驱动 → 记录（sent/failed）；异步 worker 消化
// 发送队列（SMTP IO 不占请求路径）；retry Job 扫描 pending/failed 重投
// （≤MaxRetries——无死信面）。
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/message/domain"
)

// 领域哨兵转译（repo 层哨兵 → 调用方面）。
var (
	ErrTemplateNotFound = domain.ErrTemplateNotFound
	ErrRecordNotFound   = domain.ErrRecordNotFound
	ErrChannelInvalid   = domain.ErrChannelInvalid
)

// Config 是服务参数（config.message；password 走 env Secret——bootstrap 注入）。
type Config struct {
	EmailDriver string
	SmsDriver   string
	SMTP        SMTPConfig
}

// TemplateRepo 是模板持久化契约（adapters 实现）。
type TemplateRepo interface {
	Create(ctx context.Context, t *domain.Template) error
	Update(ctx context.Context, t *domain.Template) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*domain.Template, error)
	// GetByName 按业务唯一键（name/code）取模板（不存在=domain.ErrTemplateNotFound）。
	GetByName(ctx context.Context, name string) (*domain.Template, error)
	List(ctx context.Context, name string, offset, limit int) ([]*domain.Template, int, error)
}

// RecordRepo 是发送记录持久化契约。
type RecordRepo interface {
	Create(ctx context.Context, r *domain.SendRecord) error
	UpdateStatus(ctx context.Context, id string, status int16, retries int, sendErr string) error
	Get(ctx context.Context, id string) (*domain.SendRecord, error)
	List(ctx context.Context, channel string, status int16, offset, limit int) ([]*domain.SendRecord, int, error)
	// Retryable 取待重试记录（pending/failed 且 retries<MaxRetries）。
	Retryable(ctx context.Context, limit int) ([]*domain.SendRecord, error)
	// Claim 抢占发送权（CAS：retries 期望值匹配才推进——防 worker/RetryJob 双投）。
	Claim(ctx context.Context, id string, expectedRetries int) (bool, error)
	// CountBacklogByChannel 待发送/可重试记录数（观测面——pending 指标数据源）。
	CountBacklogByChannel(ctx context.Context) (map[string]int64, error)
}

// Service 是消息服务（模板/发送/记录/驱动注册表）。
type Service struct {
	repo      *RepoPair
	inbox     InboxRepo
	registry  *Registry
	queue     chan *domain.SendRecord
	wg        sync.WaitGroup
	stop      chan struct{}
	once      sync.Once
	accessLog *slog.Logger // message access 文件面（send 流水）
	errorLog  *slog.Logger // message error 文件面（发送失败——logging.md 类型语义）
	obs       *observability.Metrics
	metrics   *Metrics // nil=不出指标（测试/未接观测面——零开销）
	now       func() time.Time
	ulid      func() string
}

// InboxMessage 是站内信（——user_id 维度隔离）。json tag snake_case
// （内与 storage/template 一致——camelCase 分裂裁决随）。
type InboxMessage struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	IsRead    bool       `json:"is_read"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
}

// InboxRepo 是站内信持久化契约（adapters 实现）。
type InboxRepo interface {
	CreateInbox(ctx context.Context, m *InboxMessage) error
	ListInbox(ctx context.Context, userID string, offset, limit int) ([]*InboxMessage, int, error)
	MarkRead(ctx context.Context, userID, id string) error
	MarkAllRead(ctx context.Context, userID string) (int64, error)
	UnreadCount(ctx context.Context, userID string) (int64, error)
}

// RepoPair 聚合 repo（AttachDB 注入；Inbox 可 nil—— 前兼容）。
type RepoPair struct {
	Templates TemplateRepo
	Records   RecordRepo
	Inbox     InboxRepo
}

// New 创建（accessLog=流水面/errorLog=失败面；obs=共享指标面，nil=不出指标；
// worker/指标挂载由 Attach 启动——模块契约）。
func New(cfg Config, accessLog, errorLog *slog.Logger, obs *observability.Metrics) *Service {
	return &Service{
		registry:  NewRegistry(cfg, accessLog),
		queue:     make(chan *domain.SendRecord, 64),
		stop:      make(chan struct{}),
		accessLog: accessLog,
		errorLog:  errorLog,
		obs:       obs,
		now:       time.Now,
		ulid: func() string {
			return primitives.NewULID(primitives.SystemClock{})
		},
	}
}

// Attach 注入 repo 并启动发送 worker（与 audit AttachDB 同惯例——goroutine
// 禁止 Init 起；pool 缺失=记录面不可用，Attach 报错 fail-fast）。
// 指标在此挂载（pending 采集器依赖 repo——scrape 时查库）。
func (s *Service) Attach(repo *RepoPair) error {
	if repo == nil || repo.Templates == nil || repo.Records == nil {
		return errors.New("message: nil repo pair")
	}
	s.repo = repo
	if repo.Inbox != nil {
		s.inbox = repo.Inbox
	}
	if s.obs != nil {
		metrics, err := NewMetrics(s.obs)
		if err != nil {
			return err
		}
		s.metrics = metrics
		if err := s.obs.Register(NewPendingCollector(repo.Records.CountBacklogByChannel, s.errorLog)); err != nil {
			return fmt.Errorf("message: register %s: %w", MetricPendingRecords, err)
		}
	}
	s.wg.Add(1)
	go s.sendLoop()
	return nil
}

// Close 关停 worker（排空队列语义：stop 关闭后 worker 退出——残留 pending
// 由 retry Job 下一周期接管）。
func (s *Service) Close() {
	s.once.Do(func() { close(s.stop) })
	s.wg.Wait()
}

// —— 模板——

// stampTemplate 写入模板主键与时间戳（创建路径共用——管理端建/自举 Ensure）。
func (s *Service) stampTemplate(t *domain.Template) {
	t.ID = s.ulid()
	t.CreatedAt, t.UpdatedAt = s.now(), s.now()
}

// CreateTemplate 创建模板（幂等冲突=ErrTemplateConflict）。
func (s *Service) CreateTemplate(ctx context.Context, t *domain.Template) error {
	if s.repo == nil {
		return domain.ErrNotReady
	}
	if !t.Channel.Valid() {
		return domain.ErrChannelInvalid
	}
	s.stampTemplate(t)
	return s.repo.Templates.Create(ctx, t)
}

// UpdateTemplate 更新模板（存在性校验 + name 不可变）。
// name 是业务唯一键（下游自举 code）：改名会让 Ensure 按原 code 新建第二行，
// 静默破坏「按 code 唯一」契约——改名=删除重建（或改文案/状态/通道）。
func (s *Service) UpdateTemplate(ctx context.Context, t *domain.Template) error {
	if s.repo == nil {
		return domain.ErrNotReady
	}
	if !t.Channel.Valid() {
		return domain.ErrChannelInvalid
	}
	existing, err := s.repo.Templates.Get(ctx, t.ID)
	if err != nil {
		return err
	}
	if t.Name != existing.Name {
		return domain.ErrTemplateNameImmutable
	}
	t.UpdatedAt = s.now()
	return s.repo.Templates.Update(ctx, t)
}

// DeleteTemplate 删除模板。
func (s *Service) DeleteTemplate(ctx context.Context, id string) error {
	if s.repo == nil {
		return domain.ErrNotReady
	}
	return s.repo.Templates.Delete(ctx, id)
}

// GetTemplate 取模板。
func (s *Service) GetTemplate(ctx context.Context, id string) (*domain.Template, error) {
	if s.repo == nil {
		return nil, domain.ErrNotReady
	}
	return s.repo.Templates.Get(ctx, id)
}

// ListTemplates 模板列表（name 包含过滤）。
func (s *Service) ListTemplates(ctx context.Context, name string, offset, limit int) ([]*domain.Template, int, error) {
	if s.repo == nil {
		return nil, 0, domain.ErrNotReady
	}
	return s.repo.Templates.List(ctx, name, offset, limit)
}

// EnsureTemplate 按 code（=模板 name，全局唯一）幂等自举模板——下游模块
// 声明式建模板（message.Templates 端口消费面），无需管理员手工预建。
//
// 语义：
//   - 不存在 → 创建（status=enabled），返回新模板；
//   - 已存在 → 原样返回，不改写内容/状态（管理端编辑与禁用为权威——
//     下游发布改名须经管理面或后续显式 Update 面）；
//   - code 已存在但通道与声明不符 → ErrTemplateSpecInvalid（fail-loud：
//     静默按错误通道发送比报错更危险）。
//
// 并发安全：多副本同时自举由 name 唯一约束兜底——Create 冲突即重读返回
// （收敛到同一行，不产生重复模板）。
func (s *Service) EnsureTemplate(ctx context.Context, code string, ch domain.Channel, subject, body string) (*domain.Template, error) {
	if s.repo == nil {
		return nil, domain.ErrNotReady
	}
	code = strings.TrimSpace(code)
	if err := validateTemplateCode(code); err != nil {
		return nil, err
	}
	if !ch.Valid() {
		return nil, fmt.Errorf("%w: channel %q", domain.ErrTemplateSpecInvalid, ch)
	}
	subject = strings.TrimSpace(subject)
	switch {
	case ch == domain.ChannelEmail && subject == "":
		return nil, fmt.Errorf("%w: email subject required", domain.ErrTemplateSpecInvalid)
	case ch == domain.ChannelSms && subject != "":
		return nil, fmt.Errorf("%w: sms subject must be empty", domain.ErrTemplateSpecInvalid)
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("%w: body required", domain.ErrTemplateSpecInvalid)
	}

	existing, err := s.repo.Templates.GetByName(ctx, code)
	switch {
	case err == nil:
		return ensureSameChannel(existing, ch)
	case !errors.Is(err, domain.ErrTemplateNotFound):
		return nil, err
	}

	tpl := &domain.Template{
		Name: code, Channel: ch, Subject: subject, Body: body,
		Status: domain.TemplateEnabled,
	}
	s.stampTemplate(tpl)
	if err := s.repo.Templates.Create(ctx, tpl); err != nil {
		if !errors.Is(err, domain.ErrTemplateConflict) {
			return nil, err
		}
		// 并发自举：另一实例已建——重读收敛（唯一约束是判定面）。
		// 重读失败时同时保留冲突与重读错误（不把 DB 故障伪装成 conflict）；
		// 端口层按哨兵映射会折叠为 conflict，故此处落 error 日志留证。
		raced, gerr := s.repo.Templates.GetByName(ctx, code)
		if gerr != nil {
			if s.errorLog != nil {
				s.errorLog.Error("message: ensure template reread failed after conflict",
					"code", code, "err", gerr)
			}
			return nil, errors.Join(err, gerr)
		}
		return ensureSameChannel(raced, ch)
	}
	if s.accessLog != nil {
		s.accessLog.Info("message.template.ensure", "template", tpl.ID, "code", code, "channel", ch)
	}
	return tpl, nil
}

// ensureSameChannel 校验既有模板通道与自举声明一致（code 复用即冲突）。
func ensureSameChannel(existing *domain.Template, want domain.Channel) (*domain.Template, error) {
	if existing.Channel != want {
		return nil, fmt.Errorf("%w: code %q exists with channel %q (want %q)",
			domain.ErrTemplateSpecInvalid, existing.Name, existing.Channel, want)
	}
	return existing, nil
}

// templateCodeMaxLen 是自举 code 长度上限（文件/日志/管理面展示安全面）。
const templateCodeMaxLen = 128

// validateTemplateCode 校验自举 code：1..128、首字符字母数字、其余 [a-z0-9._-]，
// 且至少含一个字母。code 即模板 name（管理面展示键），收窄字符集同时防空白/
// 大写漂移；「至少一个字母」阻断纯数字串（手机号等）被误用为标识的 PII 路径。
// 错误信息不回显原值（防 PII 进调用方日志），只给位置。
func validateTemplateCode(code string) error {
	if code == "" || len(code) > templateCodeMaxLen {
		return fmt.Errorf("%w: length must be 1..%d", domain.ErrTemplateCodeInvalid, templateCodeMaxLen)
	}
	hasLetter := false
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c >= 'a' && c <= 'z' {
			hasLetter = true
		}
		ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if i > 0 {
			ok = ok || c == '.' || c == '_' || c == '-'
		}
		if !ok {
			return fmt.Errorf("%w: invalid character at byte %d (lowercase [a-z0-9._-], first alphanumeric)",
				domain.ErrTemplateCodeInvalid, i)
		}
	}
	if !hasLetter {
		return fmt.Errorf("%w: must contain a letter (pure numeric codes are rejected)", domain.ErrTemplateCodeInvalid)
	}
	return nil
}

// —— 发送（send/resend + 驱动；worker 面拆 worker.go、渲染拆 render.go）——

// Send 按模板发送（admin 手动/Port 消费）：渲染 → 入队异步发送 → pending 记录。
// 返回成功=已受理（入队）；投递结果见发送记录（管理端）——worker/重试 Job 闭环。
func (s *Service) Send(ctx context.Context, templateID, target string, vars map[string]string) (*domain.SendRecord, error) {
	if s.repo == nil {
		return nil, domain.ErrNotReady
	}
	tpl, err := s.repo.Templates.Get(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if tpl.Status != domain.TemplateEnabled {
		return nil, domain.ErrTemplateDisabled
	}
	if err := s.checkDriver(tpl.Channel); err != nil {
		return nil, err
	}
	rec := &domain.SendRecord{
		ID:         s.ulid(),
		TemplateID: tpl.ID,
		Channel:    tpl.Channel,
		Target:     target,
		Subject:    render(tpl.Subject, vars),
		Body:       render(tpl.Body, vars),
		Driver:     s.activeName(tpl.Channel),
		Status:     domain.RecordPending,
		CreatedAt:  s.now(),
	}
	if err := s.repo.Records.Create(ctx, rec); err != nil {
		return nil, err
	}
	if s.accessLog != nil {
		s.accessLog.Info("message.enqueue", "record", rec.ID, "channel", rec.Channel,
			"template", rec.TemplateID, "to", rec.Target, "driver", rec.Driver)
	}
	s.enqueue(rec)
	return rec, nil
}

// Resend 重发原记录（渲染后正文重投——新记录）。
func (s *Service) Resend(ctx context.Context, recordID string) (*domain.SendRecord, error) {
	old, err := s.repo.Records.Get(ctx, recordID)
	if err != nil {
		return nil, err
	}
	rec := &domain.SendRecord{
		ID:         s.ulid(),
		TemplateID: old.TemplateID,
		Channel:    old.Channel,
		Target:     old.Target,
		Subject:    old.Subject,
		Body:       old.Body,
		Driver:     old.Driver,
		Status:     domain.RecordPending,
		CreatedAt:  s.now(),
	}
	if err := s.repo.Records.Create(ctx, rec); err != nil {
		return nil, err
	}
	s.enqueue(rec)
	return rec, nil
}

// SendTest 发送测试邮件（对齐项——管理面联调验证 SMTP 配置；
// 走同一驱动/同一超时，不落记录台账——直接用 error 返回结果）。
func (s *Service) SendTest(ctx context.Context, to string) error {
	d, err := s.registry.Active(domain.ChannelEmail)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return d.Send(ctx, domain.Outbound{
		To:      to,
		Subject: "NetyAdmin 测试邮件",
		Body:    "这是一封配置验证测试邮件（message 模块 test-email）。",
	})
}

// ApplySMTP 运行期重建 email 内置 SMTP 驱动（admin「邮件设置」保存/重置或
// configcenter 广播后的生效路径；多节点各自调用——广播保证集群收敛）。
// 仅替换同名 "smtp" 实例，不改 active 选择（下游自定义驱动 active 时不受影响）；
// 在途发送持有旧实例指针，下一次 attempt 起用新配置。
func (s *Service) ApplySMTP(cfg SMTPConfig) {
	s.registry.ReplaceDriver(domain.ChannelEmail, newSMTPDriver(cfg))
}

// ActiveDriver 返回通道当前 active 驱动名（管理面展示/诊断；未启用=空串）。
func (s *Service) ActiveDriver(ch domain.Channel) string {
	if d, err := s.registry.Active(ch); err == nil {
		return d.Name()
	}
	return ""
}

// ActiveEmailDriver 是 email 通道驱动名（cmd 组合根展示 SMTP 状态用——
// 不向 cmd 暴露 domain.Channel）。
func (s *Service) ActiveEmailDriver() string { return s.ActiveDriver(domain.ChannelEmail) }

// Records 发送记录列表。
func (s *Service) Records(ctx context.Context, channel string, status int16, offset, limit int) ([]*domain.SendRecord, int, error) {
	return s.repo.Records.List(ctx, channel, status, offset, limit)
}

// enqueue 入发送队列（满=降级同步发送？——不：记录已落 pending，队列满时
// retry Job 接管（可靠性优于请求延迟））。
func (s *Service) enqueue(rec *domain.SendRecord) {
	select {
	case s.queue <- rec:
	default: // 满——retry Job 扫 pending 重投
	}
	s.metrics.SetQueueDepth(len(s.queue))
}

// checkDriver 通道 active 驱动存在性（fail-closed）。
func (s *Service) checkDriver(ch domain.Channel) error {
	_, err := s.registry.Active(ch)
	return err
}

func (s *Service) activeName(ch domain.Channel) string {
	if d, err := s.registry.Active(ch); err == nil {
		return d.Name()
	}
	return ""
}

// —— 异步 worker + 重试（worker 面实现拆 worker.go）——

// —— 站内信——
// user_id 全部取自 authn principal（client authed 面）——永不信任请求参数。

// SendInbox 管理端发站内信（目标=user ID）。
func (s *Service) SendInbox(ctx context.Context, userID, title, body string) (*InboxMessage, error) {
	if s.inbox == nil {
		return nil, domain.ErrNotReady // 站内信 repo 未接（装配期误用）
	}
	m := &InboxMessage{
		ID: s.ulid(), UserID: userID, Title: title, Body: body, CreatedAt: s.now(),
	}
	if err := s.inbox.CreateInbox(ctx, m); err != nil {
		return nil, err
	}
	if s.accessLog != nil {
		s.accessLog.Info("message.inbox.send", "inbox", m.ID, "user", userID, "title", title)
	}
	return m, nil
}

// ListInbox 收件箱列表（本人）。
func (s *Service) ListInbox(ctx context.Context, userID string, offset, limit int) ([]*InboxMessage, int, error) {
	if s.inbox == nil {
		return nil, 0, domain.ErrNotReady // 站内信 repo 未接（装配期误用）
	}
	return s.inbox.ListInbox(ctx, userID, offset, limit)
}

// MarkRead 标记已读（user_id 双条件防越权改他人记录）。
func (s *Service) MarkRead(ctx context.Context, userID, id string) error {
	if s.inbox == nil {
		return domain.ErrNotReady // 站内信 repo 未接（装配期误用）
	}
	return s.inbox.MarkRead(ctx, userID, id)
}

// MarkAllRead 全部已读（返回标记条数）。
func (s *Service) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	if s.inbox == nil {
		return 0, domain.ErrNotReady // 站内信 repo 未接（装配期误用）
	}
	return s.inbox.MarkAllRead(ctx, userID)
}

// UnreadCount 未读计数（红点面）。
func (s *Service) UnreadCount(ctx context.Context, userID string) (int64, error) {
	if s.inbox == nil {
		return 0, domain.ErrNotReady // 站内信 repo 未接（装配期误用）
	}
	return s.inbox.UnreadCount(ctx, userID)
}
