package jwt

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims interface {
	jwt.Claims
}

// DefaultUserType is the standard userType for C-end users.
// Downstream projects define their own constants (e.g. "tech", "merchant")
// when calling NewUserClaims for custom user types.
const DefaultUserType = "user"

type AdminClaims struct {
	UserID       uint     `json:"userId"`
	Username     string   `json:"userName"`
	Roles        []string `json:"roles"`
	TokenVersion uint64   `json:"tv"` // BUG #5：签发时的 Token 版本号，中间件与 DB 当前版本比较
	jwt.RegisteredClaims
}

type UserClaims struct {
	UID              string `json:"uid"`
	Platform         string `json:"platform"`
	Type             string `json:"type"`
	TokenVersion     uint64 `json:"tv"`  // BUG #5：签发时的用户级 Token 版本号（admin 敏感操作递增 users.token_version）
	PlatTokenVersion uint64 `json:"ptv"` // 端级 Token 版本号（client Login 递增 user_tokens.token_version），同 platform 顶号
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

// JWT 使用 RS256 非对称签名：私钥签发 token，公钥验证 token。
// 私钥保留在签发端（server），公钥可下发给其他需要校验 token 的服务，
// 即使公钥泄露也无法伪造 token（仅能验证）。
type JWT struct {
	privateKey      *rsa.PrivateKey
	publicKey       *rsa.PublicKey
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// New 构造 RS256 JWT 实例。
//   - privateKey / publicKey 必须非空（fail-closed，避免无密钥启动后 token 签发/校验静默失败）
//   - accessTokenTTL / refreshTokenTTL 必须 > 0
func New(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, accessTokenTTL, refreshTokenTTL time.Duration) (*JWT, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("JWT RS256 私钥不能为空（fail-closed）")
	}
	if publicKey == nil {
		return nil, fmt.Errorf("JWT RS256 公钥不能为空（fail-closed）")
	}
	if accessTokenTTL <= 0 {
		return nil, fmt.Errorf("JWT accessTokenTTL 必须 > 0，当前值 %s", accessTokenTTL)
	}
	if refreshTokenTTL <= 0 {
		return nil, fmt.Errorf("JWT refreshTokenTTL 必须 > 0，当前值 %s", refreshTokenTTL)
	}
	return &JWT{
		privateKey:      privateKey,
		publicKey:       publicKey,
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}, nil
}

func (j *JWT) GenerateToken(claims Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(j.privateKey)
}

// expirationFor 返回指定 tokenType 的有效期：access = accessTokenTTL；refresh = refreshTokenTTL。
// 同时叠加 0~600 秒随机抖动，避免大量 token 在同一时刻集中过期引发惊群。
func (j *JWT) expirationFor(tokenType TokenType) time.Time {
	ttl := j.accessTokenTTL
	switch tokenType {
	case AccessToken:
		ttl = j.accessTokenTTL
	case RefreshToken:
		ttl = j.refreshTokenTTL
	default:
		ttl = j.accessTokenTTL
	}
	jitter := time.Duration(time.Now().UnixNano()%600) * time.Second
	return time.Now().Add(ttl).Add(jitter)
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

func (j *JWT) NewUserClaims(uid, platform, userType string, tokenType TokenType, tokenVersion, platTokenVersion uint64) *UserClaims {
	expTime := j.expirationFor(tokenType)
	return &UserClaims{
		UID:              uid,
		Platform:         platform,
		Type:             userType,
		TokenVersion:     tokenVersion,
		PlatTokenVersion: platTokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   string(tokenType),
		},
	}
}

func (j *JWT) ParseToken(tokenString string, claims Claims) error {
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// 校验签名方法必须为 RSA，防御 alg confusion 攻击：
		// 攻击者可能构造 alg=none 或 alg=HS256（用公钥作为 HMAC secret）的伪造 token，
		// 利用 RS256 公钥的 PEM 字节当作 HMAC 密钥来伪造签名。
		// 仅允许 *jwt.SigningMethodRSA（含 RS256/384/512），其余一律拒绝。
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.publicKey, nil
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
