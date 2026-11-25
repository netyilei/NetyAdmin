package primitives

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// —— JWT RS256 原语（tech-stack §1 golang-jwt/jwt/v5；策略/吊销在 kernel/security authn） ——

// ErrJWTInvalid 是验证失败的领域错误（统一口径，隐藏库内部错误细节）。
// ErrJWTExpired 是其子集（errors.Is 判定）：过期 token 与格式/签名错误分开
// 映射（10103 vs 10102； authn 消费）。
var (
	ErrJWTInvalid = errors.New("primitives: jwt invalid")
	ErrJWTExpired = fmt.Errorf("%w: expired", ErrJWTInvalid)
)

// SignJWT_RS256 用私钥签发 token。claims 为自定义声明；标准声明（exp/iat/nbf）由调用方放入。
func SignJWT_RS256(key *rsa.PrivateKey, claims map[string]any) (string, error) {
	if key == nil {
		return "", errors.New("primitives: nil private key")
	}
	m := make(jwt.MapClaims, len(claims)+1)
	for k, v := range claims {
		m[k] = v
	}
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, m)
	return t.SignedString(key)
}

// VerifyJWT_RS256 验证 RS256 token 并返回声明。
// 强制：算法白名单（RS256，防 alg=none/算法混淆）；公钥钉死。
// opts 可注入解析选项（如 jwt.WithTimeFunc(fixed) 做过期边界测试——Clock 注入口径）。
// 默认强制 exp 必须存在（无过期时间的 token 永远有效——禁止签发）。
func VerifyJWT_RS256(key *rsa.PublicKey, token string, opts ...jwt.ParserOption) (jwt.MapClaims, error) {
	if key == nil {
		return nil, errors.New("primitives: nil public key")
	}
	opts = append([]jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithTimeFunc(time.Now),
		jwt.WithExpirationRequired(),
	}, opts...)
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: 算法 %v 不在白名单", ErrJWTInvalid, t.Header["alg"])
		}
		return key, nil
	}, opts...)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %s", ErrJWTExpired, err)
		}
		return nil, fmt.Errorf("%w: %s", ErrJWTInvalid, err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, ErrJWTInvalid
	}
	return claims, nil
}
