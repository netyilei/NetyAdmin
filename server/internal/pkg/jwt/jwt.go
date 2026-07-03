package jwt

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims interface {
	jwt.Claims
}

type AdminClaims struct {
	UserID       uint     `json:"userId"`
	Username     string   `json:"userName"`
	Roles        []string `json:"roles"`
	TokenVersion uint64   `json:"tv"` // BUG #5：签发时的 Token 版本号，中间件与 DB 当前版本比较
	jwt.RegisteredClaims
}

type UserClaims struct {
	UID          string `json:"uid"`
	Platform     string `json:"platform"`
	Type         string `json:"type"`
	TokenVersion uint64 `json:"tv"` // BUG #5：签发时的 Token 版本号
	jwt.RegisteredClaims
}

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

var (
	ErrTokenExpired     = errors.New("令牌已过期")
	ErrTokenMalformed   = errors.New("令牌格式错误")
	ErrTokenInvalid     = errors.New("令牌无效")
	ErrTokenNotValidYet = errors.New("令牌尚未生效")

	// secret 强度校验正则（包级预编译，避免每次 New 重复编译）
	reLower   = regexp.MustCompile(`[a-z]`)
	reUpper   = regexp.MustCompile(`[A-Z]`)
	reDigit   = regexp.MustCompile(`[0-9]`)
	reSpecial = regexp.MustCompile(`[^a-zA-Z0-9]`)
)

// isRepeatingPattern 检测 secret 是否为单一重复字符或周期性短串
// 拒绝场景：
//   - "aaaaaaaaaaaaaaaa"        (单一字符重复)
//   - "AbcAbcAbcAbcAbcA"        (短周期 "Abc" 重复，末尾可截断)
//   - "A1A1A1A1A1A1A1A1"        (短周期 "A1" 重复)
//
// 算法：枚举可能的周期长度 p ∈ [1, 8]，若 secret 由前 p 个字符重复构成
// （最后一段允许截断）则视为弱 secret
func isRepeatingPattern(s string) bool {
	n := len(s)
	if n < 4 { // 短串不在此检查（已有长度门槛）
		return false
	}
	// 周期上限定为 8：周期 ≥9 的字符串熵已较高，可接受
	maxPeriod := 8
	if n/2 < maxPeriod {
		maxPeriod = n / 2
	}
	for p := 1; p <= maxPeriod; p++ {
		base := s[:p]
		matched := true
		for i := p; i < n; i++ {
			if s[i] != base[i%p] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

type JWT struct {
	secret     string
	expiration int
}

func New(secret string, expiration int) (*JWT, error) {
	// 校验 secret 强度：长度 ≥16 + 至少 2 类字符（小写、大写、数字、特殊符号）
	if len(secret) < 16 {
		return nil, fmt.Errorf("JWT secret 长度不足，至少需要 16 字节，当前 %d 字节", len(secret))
	}

	// 拒绝单一重复字符或周期性短串（如 "aaaaaaaaaaaaaaaa"、"AbcAbcAbcAbcAbc"）
	// 这类 secret 字符熵极低，可被字典/暴力破解快速还原
	if isRepeatingPattern(secret) {
		return nil, fmt.Errorf("JWT secret 强度不足，禁止使用单一重复字符或周期性短串")
	}

	types := 0
	if reLower.MatchString(secret) {
		types++
	}
	if reUpper.MatchString(secret) {
		types++
	}
	if reDigit.MatchString(secret) {
		types++
	}
	if reSpecial.MatchString(secret) {
		types++
	}
	if types < 2 {
		return nil, fmt.Errorf("JWT secret 强度不足，至少需要包含大小写字母、数字、特殊符号中的 2 类，当前 %d 类", types)
	}
	return &JWT{
		secret:     secret,
		expiration: expiration,
	}, nil
}

func (j *JWT) GenerateToken(claims Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secret))
}

// expirationFor 返回指定 tokenType 的有效期（access = expiration 小时；refresh = 2 倍）。
// 同时叠加 0~600 秒随机抖动，避免大量 token 在同一时刻集中过期引发惊群。
// 抽取此 helper 以消除 NewAdminClaims / NewUserClaims 间的重复（RULES.md §0.1）。
func (j *JWT) expirationFor(tokenType TokenType) time.Time {
	var hours int
	if tokenType == AccessToken {
		hours = j.expiration
	} else {
		hours = j.expiration * 2
	}
	jitter := time.Duration(time.Now().UnixNano()%600) * time.Second
	return time.Now().Add(time.Duration(hours) * time.Hour).Add(jitter)
}

func (j *JWT) NewAdminClaims(userID uint, username string, roles []string, tokenType TokenType, tokenVersion uint64) *AdminClaims {
	expTime := j.expirationFor(tokenType)
	return &AdminClaims{
		UserID:       userID,
		Username:     username,
		Roles:        roles,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   string(tokenType),
		},
	}
}

func (j *JWT) NewUserClaims(uid string, platform string, tokenType TokenType, tokenVersion uint64) *UserClaims {
	expTime := j.expirationFor(tokenType)
	return &UserClaims{
		UID:          uid,
		Platform:     platform,
		Type:         "user",
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   string(tokenType),
		},
	}
}

func (j *JWT) ParseToken(tokenString string, claims Claims) error {
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// 校验签名方法必须为 HMAC，防止 alg confusion 攻击：
		// 攻击者可能构造 alg=none 或 alg=RS256（用公钥作为 HMAC secret）的伪造 Token
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return ErrTokenExpired
		}
		return ErrTokenInvalid
	}

	if !token.Valid {
		return ErrTokenInvalid
	}

	return nil
}
