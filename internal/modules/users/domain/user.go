// user.go：users 演示模块域模型（US-01；——C 端用户能力示范，
// 下游按需扩展/替换）。PII 面=email/phone（掩码经 primitives.RegisterMasker）。
package domain

import (
	"errors"
	"time"
)

// 领域错误（调用方映射状态码）。
var (
	ErrEmailTaken         = errors.New("users: email taken")
	ErrUserNotFound       = errors.New("users: user not found")
	ErrInvalidCredentials = errors.New("users: invalid credentials")
	ErrUserDisabled       = errors.New("users: user disabled")
	ErrUsernameTaken      = errors.New("users: username taken")
	ErrPhoneTaken         = errors.New("users: phone taken")
	ErrInvalidEmail       = errors.New("users: invalid email")
	ErrInvalidPhone       = errors.New("users: invalid phone")
	ErrPasswordTooLong    = errors.New("users: password too long")
	ErrPasswordTooShort   = errors.New("users: password too short")
	ErrStatusInvalid      = errors.New("users: status invalid")
	ErrUsernameInvalid    = errors.New("users: username invalid")
)

// 用户状态。
const (
	StatusEnabled  int16 = 1
	StatusDisabled int16 = 2
)

// 用户名规则：4-32 位，字母/数字/下划线/连字符（随机生成与修改共用）。
const (
	UsernameMinLen   = 4
	UsernameMaxLen   = 32
	PasswordMaxBytes = 72 // bcrypt 块上限（identity 同口径——显式拒绝防截断）
	PasswordMinLen   = 8  // 最小长度（联测审计 D10——公开注册防 1 位密码）
)

// User 是 C 端用户（演示模块）。
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"` // PII（掩码示范——primitives.Mask）
	Username     string    `json:"username"`
	Phone        string    `json:"phone"` // PII（可选；掩码示范）
	PasswordHash string    `json:"-"`     // bcrypt hash 不出域
	Status       int16     `json:"status"`
	TokenVersion int64     `json:"token_version"` // TokenVersion（改密/注销 bump——留机制面）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ValidUsername 用户名规则校验。
func ValidUsername(name string) bool {
	if len(name) < UsernameMinLen || len(name) > UsernameMaxLen {
		return false
	}
	for _, r := range name {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '_' || r == '-'
		if !ok {
			return false
		}
	}
	return true
}

// ValidEmail 极简校验（演示模块——完整校验随邮箱验证码面后续）。
func ValidEmail(email string) bool {
	at := -1
	for i, r := range email {
		if r == '@' {
			if at >= 0 {
				return false
			}
			at = i
		}
	}
	return at > 0 && at < len(email)-1
}

// ValidPhone 极简校验（演示模块——E.164 宽松：+可选后 5-15 位数字）。
func ValidPhone(phone string) bool {
	if phone == "" {
		return true // 可选字段
	}
	if len(phone) > 1 && phone[0] == '+' {
		phone = phone[1:]
	}
	if len(phone) < 5 || len(phone) > 15 {
		return false
	}
	for _, r := range phone {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
