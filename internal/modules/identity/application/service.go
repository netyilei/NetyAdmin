// service.go：identity 应用服务（ID-01）——登录/刷新轮换/登出/改密/profile。
// 安全模型（jwt-security-model 四层 + security.md §2）：
//
//	本地验签+ 吊销面（每请求 IsRevoked）+ TokenVersion（改密全作废）
//	+ 短 TTL 兜底；refresh 一次性轮换（RotateRefresh Lua）。
//
// 登录审计：SecurityRecord 由端点层直写（产生方=接入面有
// request_id/IP/UA—— code-audit 裁决；登记接线触发点）。
// 限流：login_ip（引擎；contracts fail_open）由接入面中间件承担，服务内不重复。
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/state/session"
	"github.com/netyilei/netyadmin/internal/modules/identity/api/events"
	"github.com/netyilei/netyadmin/internal/modules/identity/domain"
)

// Config 是服务参数（config.identity 段；无具名调优诉求的常量固定——G1-15）。
type Config struct {
	AccessTTL  time.Duration // access token 寿命（<=0 取默认 2h）
	RefreshTTL time.Duration // refresh 寿命（<=0 取默认 7d）
	BcryptCost int           // security.md §2 成本参数进配置（<=0 取默认 12）
}

const (
	defaultAccessTTL  = 2 * time.Hour
	defaultRefreshTTL = 7 * 24 * time.Hour
	// passwordMaxBytes 是 bcrypt 输入上限（audit-m1 数-A5：>72 字节静默截断——
	// 显式拒绝而非截断；72 字节=bcrypt 块上限）。
	passwordMaxBytes = 72
	// passwordMinLen 是强度下限（security.md §1；V1 同口径）。
	passwordMinLen = 8
)

// 登录审计 biz_code：领域错误 → 状态码映射（application 职责；
// 单一事实源=errorx codes_gen.go——只引用常量禁手写数字；""=无业务码）。
const (
	bizCodeNone            = ""
	bizCodeLoginFailed     = errorx.CodeUnauthorized    // 10005
	bizCodeAccountDisabled = errorx.CodeAccountDisabled // 10105
)

// Service 实现 identity 端口（cmd 装配构造；pool 延迟注入）。
type Service struct {
	cfg    Config
	logger logger
	clock  primitives.Clock
	repo   domain.Repo    // 存储契约（adapters 实现）——Clean 依赖向内
	store  *session.Store // token 三面（AttachDB 构造，回退回调接 repo）
	signer Signer         // JWT 签发（keys 集成；接口解耦——测试桩）
	bus    *bus.Bus       // 登录审计事件发布（可 nil=测试；  配套）

	readyCh chan struct{} // repo/store 就绪门（AttachDB close；Init 后调用 fail-closed）
}

// logger 是最小日志面（*slog.Logger 满足；测试注入空实现）。
type logger interface {
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Signer 是 JWT 签发面（keys 集成实现；测试桩）。
type Signer interface {
	Sign(claims map[string]any) (string, error)
}

// New 创建（cfg/依赖由 module.go 装配注入）。
func New(cfg Config, log logger, clock primitives.Clock, signer Signer, eventBus *bus.Bus) *Service {
	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = defaultAccessTTL
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = defaultRefreshTTL
	}
	if cfg.BcryptCost <= 0 {
		cfg.BcryptCost = 12
	}
	if clock == nil {
		clock = primitives.SystemClock{}
	}
	// PII 掩码注册（管理面他人记录响应出口；实现与 users 同源 primitives.MaskEmail/
	// MaskPhone）。本模块独立注册：users 模块默认关闭（users.enabled=false），identity
	// 不得依赖他人注册（未注册=Mask 全文遮蔽 fail-closed，管理面将不可用）。
	primitives.RegisterMasker("email", primitives.MaskEmail, "identity")
	primitives.RegisterMasker("phone", primitives.MaskPhone, "identity")
	return &Service{cfg: cfg, logger: log, clock: clock, signer: signer, bus: eventBus,
		readyCh: make(chan struct{})}
}

// Attach 接线存储（module.go AttachDB；repo+store 构造后关闭就绪门）。
func (s *Service) Attach(repo domain.Repo, store *session.Store) {
	s.repo = repo
	s.store = store
	close(s.readyCh)
}

// ---- Port DTO ----

// LoginResult 是登录/刷新产物（token 面 + 身份投影）。
type LoginResult struct {
	AccessToken        string         `json:"access_token"`
	RefreshToken       string         `json:"refresh_token"`
	ExpiresIn          int64          `json:"expires_in"` // access 秒
	TokenType          string         `json:"token_type"`
	MustChangePassword bool           `json:"must_change_password"`
	Profile            domain.Profile `json:"profile"`
}

// ---- 登录 ----

// Login 用户名密码登录：校验 → JWT + refresh + 会话 → 审计。
// 失败恒 ErrInvalidCredentials（防枚举）；账户禁用/系统错误单独哨兵。
func (s *Service) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	admin, err := s.repo.FindByUsername(ctx, username)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		_ = primitives.ComparePassword(dummyHash, password) // 恒时序（防时序枚举）
		s.publishLoginAudit(ctx, username, "", false, bizCodeLoginFailed)
		return nil, domain.ErrInvalidCredentials
	case err != nil:
		return nil, fmt.Errorf("identity: find admin: %w", err)
	}
	if admin.Status != domain.StatusActive {
		s.publishLoginAudit(ctx, username, admin.ID, false, bizCodeAccountDisabled)
		return nil, domain.ErrAccountDisabled
	}
	if !primitives.ComparePassword(admin.PasswordHash, password) {
		s.publishLoginAudit(ctx, username, admin.ID, false, bizCodeLoginFailed)
		return nil, domain.ErrInvalidCredentials
	}
	result, err := s.issueTokens(ctx, admin)
	if err != nil {
		return nil, err
	}
	s.publishLoginAudit(ctx, username, admin.ID, true, bizCodeNone)
	if err := s.repo.TouchLastLogin(ctx, admin.ID, s.clock.Now().UTC()); err != nil {
		s.logger.Warn("identity: touch last login", "err", err)
	}
	return result, nil
}

// publishLoginAudit 发布登录审计事件（bus → audit 模块订阅落 SecurityRecord；
// security.md §2 登录成功/失败必记）。Bus Broadcast 域（3——缺省 Local
// 在拆分部署下审计事件不出节点=丢审计；对照 publishTokenRevoked 同域惯例）。
// bus 未接线/发布失败仅告警不阻断登录。
func (s *Service) publishLoginAudit(ctx context.Context, username, adminID string, success bool, bizCode string) {
	if s.bus == nil {
		return
	}
	action := "login.success"
	result := "success"
	if !success {
		action = "login.failed"
		result = "deny"
	}
	payload, err := json.Marshal(map[string]any{
		"action": action, "actor_id": adminID, "username": username,
		"result": result, "biz_code": bizCode,
	})
	if err != nil {
		return
	}
	if err := s.bus.Publish(ctx, bus.Message{Topic: events.TopicLoginAudit, Payload: payload},
		bus.WithDomain(bus.DomainBroadcast)); err != nil {
		s.logger.Error("identity: publish login audit", "err", err)
	}
}

// dummyHash 是不存在用户的恒时序比较目标（bcrypt 空耗一次防时序枚举；
// 值 = bcrypt("identity-dummy-password")，与真实口令无关）。
var dummyHash = "$2a$12$C6UzMDM.H6dfI/f/IKcEeO1QJ8Pc7KZQO3Xe5vU0q8Zm5y4T1Wz2O"

// token 吊销事件 reason 枚举（payload 契约见 events.TopicTokenRevoked 注）。
const (
	reasonPasswordChange  = "password_change"
	reasonPasswordReset   = "password_reset"
	reasonAccountDisabled = "account_disabled"
)

// publishTokenRevoked 发布 token 全作废事件（TokenVersion bump 成功后调用；
// Broadcast 域——multi 下他节点即时感知会话作废，缺省 Local 不出节点）。
// bus 未接线/发布失败仅告警不阻断：DB token_version 是作废唯一真相源，
// 事件是传播优化（订阅方可周期对账，configcenter.broadcast 同判定）。
func (s *Service) publishTokenRevoked(ctx context.Context, adminID, reason string) {
	if s.bus == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{"admin_id": adminID, "reason": reason})
	if err != nil {
		return
	}
	if err := s.bus.Publish(ctx, bus.Message{Topic: events.TopicTokenRevoked, Payload: payload},
		bus.WithDomain(bus.DomainBroadcast)); err != nil {
		s.logger.Error("identity: publish token revoked", "err", err)
	}
}

// ---- token 签发/轮换 ----

func (s *Service) issueTokens(ctx context.Context, admin domain.Admin) (*LoginResult, error) {
	jti := primitives.NewULID(s.clock)
	now := s.clock.Now()
	sessID := primitives.NewULID(s.clock)
	access, err := s.signer.Sign(map[string]any{
		authn.ClaimSub:          admin.ID,
		authn.ClaimJTI:          jti,
		"sid":                   sessID, // 会话绑定：登出经 sid 杀 refresh 链（提取）
		authn.ClaimTokenVersion: admin.TokenVersion,
		"iat":                   now.Unix(),
		"exp":                   now.Add(s.cfg.AccessTTL).Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("identity: sign access token: %w", err)
	}
	refresh := sessID + "." + primitives.NewULID(s.clock) // 自编码 sessionID（Refresh 解析）
	snapshot := domain.SessionSnapshot{
		AdminID: admin.ID, Username: admin.Username, TokenVer: admin.TokenVersion,
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("identity: marshal session: %w", err)
	}
	if err := s.store.PutSession(ctx, sessID, payload, s.cfg.RefreshTTL); err != nil {
		return nil, mapStoreErr(err)
	}
	if err := s.store.BindRefresh(ctx, refresh, sessID, s.cfg.RefreshTTL); err != nil {
		return nil, mapStoreErr(err)
	}
	// 会话回退副本落库（session_store read_fallback: db 读源）
	if err := s.repo.PutSessionFallback(ctx, sessID, admin.ID, payload, now.Add(s.cfg.RefreshTTL)); err != nil {
		s.logger.Warn("identity: persist session fallback", "err", err)
	}
	return &LoginResult{
		AccessToken: access, RefreshToken: refresh,
		ExpiresIn: int64(s.cfg.AccessTTL.Seconds()), TokenType: "Bearer",
		MustChangePassword: admin.MustChangePassword, Profile: admin.ToProfile(),
	}, nil
}

// Refresh 刷新轮换：旧 refresh 一次性消费 → 新 token 对。
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	// refresh token 自编码 sessionID（<sessID>.<rand>；issueTokens 生成）——
	// Store 无 refresh→session 反查面，编码式推导省一次查询。
	sessID, ok := parseRefresh(refreshToken)
	if !ok {
		return nil, domain.ErrSessionDenied
	}
	newRefresh := sessID + "." + primitives.NewULID(s.clock)
	if err := s.store.RotateRefresh(ctx, refreshToken, newRefresh, sessID, s.cfg.RefreshTTL); err != nil {
		return nil, mapStoreErr(err) // ErrDenied → ErrSessionDenied（一次性语义）
	}
	payload, found, err := s.store.GetSession(ctx, sessID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	if !found {
		return nil, domain.ErrSessionDenied
	}
	snap := domain.SessionSnapshot{}
	if err := json.Unmarshal(payload, &snap); err != nil {
		return nil, domain.ErrSessionDenied
	}
	admin, err := s.repo.FindByID(ctx, snap.AdminID)
	if err != nil {
		return nil, domain.ErrSessionDenied // 会话指向的账户已不存在=会话作废
	}
	if admin.Status != domain.StatusActive {
		return nil, domain.ErrAccountDisabled
	}
	// TokenVersion 快照校验：会话版本落后当前（改密/封禁触发 ++）→ 旧会话失效
	if snap.TokenVer != admin.TokenVersion {
		_ = s.store.DropSession(ctx, sessID)
		return nil, domain.ErrSessionDenied
	}
	return s.issueTokens(ctx, admin)
}

// parseRefresh 解析自编码 refresh（sessID.rand）。
func parseRefresh(token string) (string, bool) {
	sessID, rand, ok := strings.Cut(token, ".")
	if !ok || sessID == "" || rand == "" {
		return "", false
	}
	return sessID, true
}

// ---- 登出/改密/Profile ----

// Logout 吊销 access jti（TTL=剩余寿命）+ 丢弃会话。
func (s *Service) Logout(ctx context.Context, jti string, accessExp time.Time, sessID string) error {
	if err := s.ready(); err != nil {
		return err
	}
	ttl := accessExp.Sub(s.clock.Now())
	if ttl <= 0 {
		ttl = time.Minute
	}
	if err := s.store.Revoke(ctx, jti, ttl); err != nil {
		return mapStoreErr(err)
	}
	// DB 读源同步落库（single 拓扑 Redis 宕机时回退读覆盖已登出 token）
	if err := s.repo.RevokePersisted(ctx, jti, s.clock.Now().UTC().Add(ttl)); err != nil {
		s.logger.Warn("identity: persist revocation", "err", err)
	}
	if sessID != "" {
		if err := s.store.DropSession(ctx, sessID); err != nil {
			return mapStoreErr(err)
		}
		if err := s.repo.DeleteSessionFallback(ctx, sessID); err != nil {
			return mapStoreErr(err) // 回退副本不删=已登出会话经 DB 回退复活
		}
	}
	return nil
}

// ChangePassword 改密：旧密码校验 → 新 hash → TokenVersion++ 全作废。
func (s *Service) ChangePassword(ctx context.Context, adminID, oldPassword, newPassword string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if len(newPassword) < passwordMinLen || len(newPassword) > passwordMaxBytes {
		return domain.ErrPasswordWeak
	}
	admin, err := s.repo.FindByID(ctx, adminID)
	if err != nil {
		return err
	}
	if !primitives.ComparePassword(admin.PasswordHash, oldPassword) {
		return domain.ErrInvalidCredentials
	}
	hash, err := primitives.HashPassword(newPassword, s.cfg.BcryptCost)
	if err != nil {
		return fmt.Errorf("identity: hash password: %w", err)
	}
	if err := s.repo.UpdatePassword(ctx, adminID, hash, true, false); err != nil { // bumpVersion 全作废
		return err
	}
	s.publishTokenRevoked(ctx, adminID, reasonPasswordChange)
	return nil
}

// GetProfile 展示投影。
func (s *Service) GetProfile(ctx context.Context, adminID string) (domain.Profile, error) {
	if err := s.ready(); err != nil {
		return domain.Profile{}, err
	}
	admin, err := s.repo.FindByID(ctx, adminID)
	if err != nil {
		return domain.Profile{}, err
	}
	return admin.ToProfile(), nil
}

// UpdateAdminFace 管理面编辑（name/email/phone——email/phone 指针 nil=不改该列，
// 防编辑表单未提交字段清空；email 空串=显式清除。 扩展）。
// 响应走管理面出口掩码（操作对象是他人记录）。
func (s *Service) UpdateAdminFace(ctx context.Context, adminID, name string, email, phone *string) (domain.Profile, error) {
	if err := s.ready(); err != nil {
		return domain.Profile{}, err
	}
	if err := s.repo.UpdateAdminFace(ctx, adminID, name, email, phone); err != nil {
		return domain.Profile{}, err
	}
	p, err := s.GetProfile(ctx, adminID)
	if err != nil {
		return domain.Profile{}, err
	}
	return maskProfile(p), nil
}

// UpdateProfile 更新展示字段。
func (s *Service) UpdateProfile(ctx context.Context, adminID, name, email, phone, avatarURL string) (domain.Profile, error) {
	if err := s.ready(); err != nil {
		return domain.Profile{}, err
	}
	if len(avatarURL) > 200*1024 {
		return domain.Profile{}, domain.ErrAvatarTooLarge // 400 语义（前端提示压缩）
	}
	if err := s.repo.UpdateProfile(ctx, adminID, name, email, phone, avatarURL); err != nil {
		return domain.Profile{}, err
	}
	return s.GetProfile(ctx, adminID)
}

// ---- 内部 ----

func (s *Service) ready() error {
	select {
	case <-s.readyCh: // AttachDB 后 close
		return nil
	default:
		return errors.New("identity: storage not attached (fail-closed)")
	}
}

func mapStoreErr(err error) error {
	switch {
	case errors.Is(err, session.ErrDenied):
		return domain.ErrSessionDenied
	case errors.Is(err, session.ErrUnavailable):
		return fmt.Errorf("identity: %w: %w", domain.ErrStoreUnavailable, err)
	default:
		return fmt.Errorf("identity: token store: %w", err)
	}
}
