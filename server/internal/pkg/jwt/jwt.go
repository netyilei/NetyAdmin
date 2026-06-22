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
	UserID   uint     `json:"userId"`
	Username string   `json:"userName"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

type UserClaims struct {
	UID      string `json:"uid"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
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
)

type JWT struct {
	secret     string
	expiration int
}

func New(secret string, expiration int) (*JWT, error) {
	// 校验 secret 强度：长度 ≥16 + 至少 2 类字符（小写、大写、数字、特殊符号）
	if len(secret) < 16 {
		return nil, fmt.Errorf("JWT secret 长度不足，至少需要 16 字节，当前 %d 字节", len(secret))
	}
	types := 0
	if matched, _ := regexp.MatchString(`[a-z]`, secret); matched {
		types++
	}
	if matched, _ := regexp.MatchString(`[A-Z]`, secret); matched {
		types++
	}
	if matched, _ := regexp.MatchString(`[0-9]`, secret); matched {
		types++
	}
	if matched, _ := regexp.MatchString(`[^a-zA-Z0-9]`, secret); matched {
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

func (j *JWT) NewAdminClaims(userID uint, username string, roles []string, tokenType TokenType) *AdminClaims {
	var expDuration time.Duration
	if tokenType == AccessToken {
		expDuration = time.Duration(j.expiration) * time.Hour
	} else {
		expDuration = time.Duration(j.expiration*2) * time.Hour
	}

	jitter := time.Duration(time.Now().UnixNano()%600) * time.Second
	expTime := time.Now().Add(expDuration).Add(jitter)

	return &AdminClaims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   string(tokenType),
		},
	}
}

func (j *JWT) NewUserClaims(uid string, platform string, tokenType TokenType) *UserClaims {
	var expDuration time.Duration
	if tokenType == AccessToken {
		expDuration = time.Duration(j.expiration) * time.Hour
	} else {
		expDuration = time.Duration(j.expiration*2) * time.Hour
	}

	jitter := time.Duration(time.Now().UnixNano()%600) * time.Second
	expTime := time.Now().Add(expDuration).Add(jitter)

	return &UserClaims{
		UID:      uid,
		Platform: platform,
		Type:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   string(tokenType),
		},
	}
}

func (j *JWT) ParseToken(tokenString string, claims Claims) error {
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
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
