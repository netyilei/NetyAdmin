// service.go：users 演示服务（US-01）——注册（邮箱+密码→随机用户
// 名）/登录（User JWT， 签发+ TokenVersion）/改用户名/改手机号。
// PII 掩码示范：email/phone 经 primitives.RegisterMasker 注册（下游可覆盖）。
// 密码=bcrypt cost 12（identity 同口径）；恒时序防枚举（dummy hash）。
package application

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/users/domain"
)

// 领域哨兵（repo 层转译）。
var (
	ErrEmailTaken         = domain.ErrEmailTaken
	ErrUserNotFound       = domain.ErrUserNotFound
	ErrInvalidCredentials = domain.ErrInvalidCredentials
	ErrUserDisabled       = domain.ErrUserDisabled
	ErrUsernameTaken      = domain.ErrUsernameTaken
	ErrPhoneTaken         = domain.ErrPhoneTaken
	ErrInvalidEmail       = domain.ErrInvalidEmail
	ErrInvalidPhone       = domain.ErrInvalidPhone
	ErrUsernameInvalid    = domain.ErrUsernameInvalid
	ErrPasswordTooLong    = domain.ErrPasswordTooLong
)

// DefaultAccessTokenTTL 是 User access token 寿命基线（clientapi expires_in
// 输出面同源——G1-08 具名）。
const DefaultAccessTokenTTL = 2 * time.Hour

// Config 是服务参数（config.users——演示模块默认值即基线）。
type Config struct {
	AccessTokenTTL time.Duration // 默认 2h
	BcryptCost     int           // 默认 12
	Issuer         string        // JWT iss（默认 netyadmin）
	Audience       string        // JWT aud（默认 client）
}

// UserRepo 是用户持久化契约（adapters 实现）。
type UserRepo interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByPhone(ctx context.Context, phone string) (*domain.User, error)
	UpdateUsername(ctx context.Context, id, username string) error
	UpdatePhone(ctx context.Context, id, phone string) error
	SetStatus(ctx context.Context, id string, status int16) error
	SetTokenVersion(ctx context.Context, id string, version int64) error
	List(ctx context.Context, offset, limit int) ([]*domain.User, int, error)
}

// dummyHash 是不存在用户的恒时序比较目标（bcrypt 空耗一次防时序枚举——
// identity 同口径）。
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("users-dummy"), 12)

// emailPattern 是极简邮箱校验（domain.ValidEmail 同口径编译缓存）。
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Service 是 users 演示服务。
type Service struct {
	repo      UserRepo
	signer    *rsa.PrivateKey // 签发（多公钥轮换共用 SigningKeys——cmd 注入）
	logger    *slog.Logger
	cfg       Config
	now       func() time.Time
	ulid      func() string
	accessLog *slog.Logger
}

// New 创建并注册 PII 掩码示范（email/phone——下游可覆盖 RegisterMasker）。
func New(cfg Config, signer *rsa.PrivateKey, accessLog, logger *slog.Logger) *Service {
	if cfg.AccessTokenTTL <= 0 {
		cfg.AccessTokenTTL = DefaultAccessTokenTTL
	}
	if cfg.BcryptCost <= 0 {
		cfg.BcryptCost = 12
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "netyadmin"
	}
	if cfg.Audience == "" {
		cfg.Audience = "client"
	}
	// PII 掩码注册示范（US-01 验收「PII 掩码注册生效」——owner=users，下游覆盖；
	// 实现收拢在 primitives.MaskEmail/MaskPhone——identity 管理面同源复用）
	primitives.RegisterMasker("email", primitives.MaskEmail, "users")
	primitives.RegisterMasker("phone", primitives.MaskPhone, "users")
	return &Service{
		logger: logger, cfg: cfg, signer: signer, accessLog: accessLog,
		now:  time.Now,
		ulid: func() string { return primitives.NewULID(primitives.SystemClock{}) },
	}
}

// Attach 注入 repo（DB 连接后——模块契约）。
func (s *Service) Attach(repo UserRepo) { s.repo = repo }

func (s *Service) repoOK() error {
	if s.repo == nil {
		return errors.New("users: repo not attached")
	}
	return nil
}

// randomUsername 生成随机用户名（user_ + 单次 ULID 末 8 位转小写——随机源
// =ULID crypto；碰撞概率与查重兜底并存）。
func randomUsername() string {
	id := primitives.NewULID(primitives.SystemClock{})
	tail := strings.ToLower(id[len(id)-8:])
	return "user_" + tail
}

// Register 注册（用户需求：邮箱+密码 → 随机用户名；返回掩码视图）。
func (s *Service) Register(ctx context.Context, email, password string) (*domain.User, error) {
	if err := s.repoOK(); err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailPattern.MatchString(email) {
		return nil, domain.ErrInvalidEmail
	}
	if len(password) > domain.PasswordMaxBytes {
		return nil, domain.ErrPasswordTooLong
	}
	if len(password) < domain.PasswordMinLen {
		return nil, domain.ErrPasswordTooShort // D10——公开注册防 1 位密码
	}
	if _, err := s.repo.GetByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cfg.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("users: bcrypt: %w", err)
	}
	u := &domain.User{
		ID:           s.ulid(),
		Email:        email,
		Username:     randomUsername(),
		PasswordHash: string(hash),
		Status:       domain.StatusEnabled,
		TokenVersion: 1, // 基线（零值覆盖迁移 default 漂移）
		CreatedAt:    s.now(),
		UpdatedAt:    s.now(),
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := s.repo.Create(ctx, u); err == nil {
			return u, nil
		} else if !errors.Is(err, domain.ErrUsernameTaken) {
			return nil, err
		}
		u.Username = randomUsername() // 随机名撞唯一约束 → 换名重试（查重兜底真实化）
		u.UpdatedAt = s.now()
	}
	return nil, domain.ErrUsernameTaken
}

// Login 登录（恒时序防枚举 → User JWT 签发）。
func (s *Service) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	if err := s.repoOK(); err != nil {
		return nil, "", err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if u.Status != domain.StatusEnabled {
		return nil, "", ErrUserDisabled
	}
	token, err := s.issueToken(u)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

// issueToken 签发 User JWT（签发面共用 SigningKeys——claims
// sub/jti/tv/iat/exp/iss/aud；吊销面=TokenVersion per-row， 模式）。
func (s *Service) issueToken(u *domain.User) (string, error) {
	now := s.now()
	claims := map[string]any{
		"sub": u.ID,
		"jti": s.ulid(),
		"tv":  u.TokenVersion,
		"iat": now.Unix(),
		"exp": now.Add(s.cfg.AccessTokenTTL).Unix(),
		"iss": s.cfg.Issuer,
		"aud": s.cfg.Audience,
	}
	token, err := primitives.SignJWT_RS256(s.signer, claims)
	if err != nil {
		return "", fmt.Errorf("users: sign token: %w", err)
	}
	return token, nil
}

// UpdateUsername 修改用户名（登录后；查重）。
func (s *Service) UpdateUsername(ctx context.Context, principalID, username string) error {
	if err := s.repoOK(); err != nil {
		return err
	}
	if !domain.ValidUsername(username) {
		return domain.ErrUsernameInvalid
	}
	if u, err := s.repo.GetByUsername(ctx, username); err == nil && u.ID != principalID {
		return ErrUsernameTaken // D5：查重排除自身（设为当前值=幂等而非冲突）
	}
	if err := s.repo.UpdateUsername(ctx, principalID, username); err != nil {
		return err
	}
	return nil
}

// UpdatePhone 修改手机号（登录后；查重+可空）。
func (s *Service) UpdatePhone(ctx context.Context, principalID, phone string) error {
	if err := s.repoOK(); err != nil {
		return err
	}
	if !domain.ValidPhone(phone) {
		return domain.ErrInvalidPhone
	}
	if phone != "" {
		if u, err := s.repo.GetByPhone(ctx, phone); err == nil && u.ID != principalID {
			return ErrPhoneTaken // D5：查重排除自身
		}
	}
	return s.repo.UpdatePhone(ctx, principalID, phone)
}

// Profile 用户视图（PII 掩码示范——phone/email 经 primitives.Mask）。
func (s *Service) Profile(ctx context.Context, principalID string) (*domain.User, error) {
	if err := s.repoOK(); err != nil {
		return nil, err
	}
	u, err := s.repo.GetByID(ctx, principalID)
	if err != nil {
		return nil, err
	}
	u.Email = primitives.Mask("email", u.Email)
	if u.Phone != "" {
		u.Phone = primitives.Mask("phone", u.Phone)
	}
	return u, nil
}

// GetUser 管理端单用户详情（PII 掩码——与 Profile/ListUsers 同口径；US-01）。
func (s *Service) GetUser(ctx context.Context, id string) (*domain.User, error) {
	if err := s.repoOK(); err != nil {
		return nil, err
	}
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u.Email = primitives.Mask("email", u.Email)
	if u.Phone != "" {
		u.Phone = primitives.Mask("phone", u.Phone)
	}
	return u, nil
}

// TokenVersionSource 是 authn BaseOptions.VersionSource（per-row 版本）。
func (s *Service) TokenVersionSource(ctx context.Context, principalID string) (int64, error) {
	u, err := s.repo.GetByID(ctx, principalID)
	if err != nil {
		return 0, err
	}
	return u.TokenVersion, nil
}

// ListUsers 管理端列表（email/phone 掩码输出）。
func (s *Service) ListUsers(ctx context.Context, offset, limit int) ([]*domain.User, int, error) {
	if err := s.repoOK(); err != nil {
		return nil, 0, err
	}
	list, total, err := s.repo.List(ctx, offset, limit)
	for _, u := range list {
		u.Email = primitives.Mask("email", u.Email)
		if u.Phone != "" {
			u.Phone = primitives.Mask("phone", u.Phone)
		}
	}
	return list, total, err
}

// SetStatus 管理端禁用/启用。
func (s *Service) SetStatus(ctx context.Context, id string, status int16) error {
	if err := s.repoOK(); err != nil {
		return err
	}
	if status != domain.StatusEnabled && status != domain.StatusDisabled {
		return domain.ErrStatusInvalid // D2 域校验（原任意值落库；DB CHECK 双闸）
	}
	if status == domain.StatusDisabled {
		// 禁用=bump token_version（——现行 JWT 立即失效，不等 TTL）
		u, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if err := s.repo.SetTokenVersion(ctx, id, u.TokenVersion+1); err != nil {
			return err
		}
	}
	return s.repo.SetStatus(ctx, id, status)
}
