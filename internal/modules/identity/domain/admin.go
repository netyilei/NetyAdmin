// admin.go：管理员领域模型与错误（ID-01；identity 收缩纯 admin——30）。
// 业务判定值与展示值分离：PasswordHash 永不出 domain（Profile 返回脱敏实体）。
package domain

import (
	"context"
	"errors"
	"time"
)

// Status 是账户状态（admins.status smallint 镜像）。
type Status int8

const (
	StatusActive   Status = 1
	StatusDisabled Status = 2
)

// Valid 校验状态枚举（数-A4 清偿：筛选面非法值=400 而非恒空集）。
// 0=不过滤（ListFilter 零值语义）。
func (s Status) Valid() bool { return s == StatusActive || s == StatusDisabled }

// 领域错误（调用方映射状态码；「领域错误→状态码」映射在 application 层
// （——domain 禁 import errorx）；消息不回显到客户端的细节在 Error 包装内）。
var (
	// ErrInvalidCredentials：用户名或密码错误（防枚举——单一错误面）。
	ErrInvalidCredentials = errors.New("identity: invalid credentials")
	// ErrAccountDisabled：账户禁用（10105）。
	ErrAccountDisabled = errors.New("identity: account disabled")
	// ErrNotFound：账户不存在（管理面用；登录面恒 ErrInvalidCredentials 防枚举）。
	ErrNotFound = errors.New("identity: admin not found")
	// ErrAvatarTooLarge：头像 data URL 超 200KB（个人资料——前端压缩提示）。
	ErrAvatarTooLarge = errors.New("identity: avatar too large")
	// ErrPhoneInvalid：手机号格式非法（管理员编辑/资料修改）。
	ErrPhoneInvalid = errors.New("identity: phone invalid")
	// ErrUsernameTaken：用户名已占用（管理面创建）。
	ErrUsernameTaken = errors.New("identity: username taken")
	// ErrPasswordWeak：密码不满足强度（登录面限长 security.md）。
	ErrPasswordWeak = errors.New("identity: password too weak")
	// ErrSessionDenied：refresh 轮换被拒（一次性占用失败/会话不匹配—— ErrDenied）。
	ErrSessionDenied = errors.New("identity: session denied")
	// ErrStoreUnavailable：token 面系统错误透传（session.ErrUnavailable 链——调用方 10010）。
	ErrStoreUnavailable = errors.New("identity: token store unavailable")
	// ErrSelfOperation：操作者对自身执行危险管理动作（禁用/删除——防自锁）。
	ErrSelfOperation = errors.New("identity: self operation denied")
)

// Admin 是管理员实体（表 admins 镜像；PasswordHash 仅登录校验内部可见）。
type Admin struct {
	ID                 string
	Username           string
	PasswordHash       string // 永不外发（Profile 输出剥离）
	Name               string
	Email              string
	Phone              string // PII（掩码归出口）
	AvatarURL          string // 头像（data URL 或对象存储 URL——个人资料）
	Status             Status
	MustChangePassword bool
	TokenVersion       int64
	LastLoginAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Profile 是对外展示投影（剥离 PasswordHash）。PII 出口口径：
// 登录者看自己（GetProfile/UpdateProfile/LoginResult）明文；管理面他人记录
// 列表/创建/编辑响应经 application.maskProfile 掩码（email/phone），详情
// （GetAdmin，admin.admins.get 权限）返回原文供编辑表单回填（禁掩码串回写）。
type Profile struct {
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	Name               string     `json:"name"`
	Email              string     `json:"email"`
	Phone              string     `json:"phone"`
	AvatarURL          string     `json:"avatarUrl"`
	Status             Status     `json:"status"`
	MustChangePassword bool       `json:"mustChangePassword"`
	LastLoginAt        *time.Time `json:"lastLoginAt"`
}

// ToProfile 生成展示投影（唯一出口——防 hash 外泄的结构性收口）。
func (a Admin) ToProfile() Profile {
	return Profile{
		ID:                 a.ID,
		Username:           a.Username,
		Name:               a.Name,
		Email:              a.Email,
		Phone:              a.Phone,
		AvatarURL:          a.AvatarURL,
		Status:             a.Status,
		MustChangePassword: a.MustChangePassword,
		LastLoginAt:        a.LastLoginAt,
	}
}

// SessionSnapshot 是会话负载契约（session payload 的 jsonb 序列化结构）。
type SessionSnapshot struct {
	AdminID  string   `json:"admin_id"`
	Username string   `json:"username"`
	TokenVer int64    `json:"token_version"`
	Roles    []string `json:"roles"`
}

// ListFilter 是管理面列表过滤（零值字段=不过滤，Status 零值=全部）。
type ListFilter struct {
	Username string // 前缀匹配（LIKE 已转义）
	Name     string // 包含匹配（LIKE 已转义）
	Status   Status // 0=全部
	Offset   int
	Limit    int
}

// Repo 是存储契约（application 定义、adapters 实现——Clean 架构依赖向内；
// 方法集=Service 全部存储需求）。
type Repo interface {
	FindByUsername(ctx context.Context, username string) (Admin, error)
	FindByID(ctx context.Context, id string) (Admin, error)
	TouchLastLogin(ctx context.Context, id string, at time.Time) error
	// UpdatePassword 改密/重置：bumpVersion=用户级全作废；mustChange=强制首登改密
	// （bootstrap/管理员重置 true、自助改密 false）。
	UpdatePassword(ctx context.Context, id, newHash string, bumpVersion, mustChange bool) error
	// UpdateAdminFace 管理面编辑；email/phone 指针 nil=不改该列（空串=显式清除）。
	UpdateAdminFace(ctx context.Context, id, name string, email *string, phone *string) error
	UpdateProfile(ctx context.Context, id, name, email, phone, avatarURL string) error
	SetStatus(ctx context.Context, id string, status Status, bumpVersion bool) error
	CountAdmins(ctx context.Context) (int, error)
	InsertAdmin(ctx context.Context, a Admin) error
	List(ctx context.Context, f ListFilter) ([]Admin, int64, error)
	Delete(ctx context.Context, ids []string) error
	// DB 回退面（读向语义；写向双写由 Service 编排）
	RevokePersisted(ctx context.Context, jti string, expiresAt time.Time) error
	IsRevokedFallback(ctx context.Context, jti string) (bool, error)
	PutSessionFallback(ctx context.Context, sessionID, adminID string, payload []byte, expiresAt time.Time) error
	GetSessionFallback(ctx context.Context, sessionID string) ([]byte, bool, error)
	DeleteSessionFallback(ctx context.Context, sessionID string) error
	// CleanExpiredRevocations 清理过期吊销行（expires_at < before；周期 Job 容量
	// 维护——过期行 IsRevokedFallback 恒 false，删除不触安全语义面）。
	CleanExpiredRevocations(ctx context.Context, before time.Time) (int64, error)
}
