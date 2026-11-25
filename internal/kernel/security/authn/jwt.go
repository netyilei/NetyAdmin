// jwt.go：JWT RS256 验证链——JWT 安全模型三层 + TTL 兜底的机制实现：
//
//	① 本地验签（真伪， 原语；多公钥 = 轮换重叠窗口任一通过）
//	② 吊销面（生死， IsRevoked 每请求；fail-closed 语义在 session 包）
//	③ TokenVersion（用户级全作废，源由 ID-01 注入）
//	④ exp 强制（primitives WithExpirationRequired）
//
// admin/user 通用（service 专属契约见 service.go；app HMAC 见 hmac.go）。
package authn

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
)

// 标准 claim 键（签发方契约；接收方按此解析，缺声明按 fail-closed 拒绝）。
const (
	ClaimSub          = "sub"
	ClaimJTI          = "jti"
	ClaimRoles        = "roles"
	ClaimPlatform     = "platform"
	ClaimScopes       = "scp" // 权威键名（契约测试实证：TR-02 client 签 scp /verifier 读常量——键名不一致=真实调用全 401，stub 测试掩盖）
	ClaimAct          = "act"
	ClaimTokenVersion = "tv"
	ClaimSessionID    = "sid" // 会话绑定（identity  签发；登出经 sid 杀会话）
)

// TokenRevocation 是吊销面（每请求查 jti 是否已作废）。
// *session.Store 结构性满足；窄接口防 authn↔session 硬耦合。
type TokenRevocation interface {
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// VersionSource 是 TokenVersion 查询源（当前有效版本）；查询失败按错误透传
// （fail-closed：拒绝而非放行）。 ID-01 接表注入；nil + CheckTokenVersion = 配置错误。
type VersionSource func(ctx context.Context, principalID string) (int64, error)

// BaseOptions 是 JWT 验证共享参数（admin/user/service 三类复用）。
type BaseOptions struct {
	// PublicKeys 验签公钥集：任一通过即有效（security.md §1 轮换重叠窗口）。
	PublicKeys []*rsa.PublicKey
	Issuer     string // 空 = 不校验 iss
	Audience   string // 空 = 不校验 aud
	Leeway     time.Duration
	// TimeFunc 时间注入（测试过期边界用；nil = time.Now）。
	TimeFunc func() time.Time

	CheckRevocation   bool            // ② 吊销面开关（模型要求开启；机制允许关闭由使用方自担）
	Revocation        TokenRevocation // session.Store
	CheckTokenVersion bool            // ③ TokenVersion 开关
	VersionSource     VersionSource
}

func (o BaseOptions) validate() error {
	if len(o.PublicKeys) == 0 {
		return errors.New("authn: no public keys")
	}
	for _, k := range o.PublicKeys {
		if k == nil {
			return errors.New("authn: nil public key")
		}
	}
	if o.CheckRevocation && o.Revocation == nil {
		return errors.New("authn: CheckRevocation requires Revocation")
	}
	if o.CheckTokenVersion && o.VersionSource == nil {
		return errors.New("authn: CheckTokenVersion requires VersionSource")
	}
	return nil
}

func (o BaseOptions) parserOpts() []jwt.ParserOption {
	var opts []jwt.ParserOption
	if o.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(o.Issuer))
	}
	if o.Audience != "" {
		opts = append(opts, jwt.WithAudience(o.Audience))
	}
	if o.Leeway > 0 {
		opts = append(opts, jwt.WithLeeway(o.Leeway))
	}
	if o.TimeFunc != nil {
		opts = append(opts, jwt.WithTimeFunc(o.TimeFunc)) // 覆盖 primitives 默认（后写胜出）
	}
	return opts
}

// verify 步骤①：多公钥逐试验签，并把 primitives 哨兵翻译为 authn 领域哨兵
// （过期优先归类：任一密钥报告过期即签名已验真、token 真过期；其余 → ErrInvalidToken）。
func (o BaseOptions) verify(_ context.Context, cred Credential) (jwt.MapClaims, error) {
	if cred.Token == "" {
		return nil, ErrNoCredential
	}
	var (
		lastErr error
		expired bool
	)
	for _, key := range o.PublicKeys {
		claims, err := primitives.VerifyJWT_RS256(key, cred.Token, o.parserOpts()...)
		if err == nil {
			return claims, nil
		}
		if errors.Is(err, primitives.ErrJWTExpired) {
			expired = true // jwt/v5 签名验证先于 claims 校验：过期结论仅在签名通过时产生
		}
		lastErr = err
	}
	if expired {
		return nil, ErrExpiredToken
	}
	return nil, fmt.Errorf("%w: %w", ErrInvalidToken, lastErr)
}

// enforce 步骤②③：吊销面 + TokenVersion。系统错误（Redis 不可用/源查询失败）
// 包装透传——调用方拿非 nil error 即拒绝，errors.Is(session.ErrUnavailable) 映射 10010。
func (o BaseOptions) enforce(ctx context.Context, id, jti string, tokenVersion int64, hasTV bool) error {
	if o.CheckRevocation {
		if jti == "" {
			return fmt.Errorf("%w: missing %s", ErrInvalidToken, ClaimJTI)
		}
		revoked, err := o.Revocation.IsRevoked(ctx, jti)
		if err != nil {
			return fmt.Errorf("authn: revocation check: %w", err)
		}
		if revoked {
			return ErrRevokedToken
		}
	}
	if o.CheckTokenVersion {
		if !hasTV {
			return fmt.Errorf("%w: missing %s claim", ErrInvalidToken, ClaimTokenVersion)
		}
		current, err := o.VersionSource(ctx, id)
		if err != nil {
			return fmt.Errorf("authn: version source: %w", err)
		}
		if current != tokenVersion {
			return ErrRevokedToken // 用户级全作废：旧版本 token 一律拒绝
		}
	}
	return nil
}

// —— claim 解析帮助（MapClaims 反序列化数值均为 float64） ——

func claimsString(claims jwt.MapClaims, key string) (string, bool) {
	v, ok := claims[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func claimsStrings(claims jwt.MapClaims, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func claimsInt64(claims jwt.MapClaims, key string) (int64, bool) {
	v, ok := claims[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}

// JWTVerifier 是 admin/user JWT Provider（§8.1 Admin JWT / User JWT 行）。
// 接入面各自持有实例（adminapi 挂 admin 配置，clientapi 挂 user 配置），
// scheme 同为 "bearer"——一个 Chain 只装一个 bearer（接入面组合语义）。
type JWTVerifier struct {
	ptype PrincipalType
	base  BaseOptions
}

// NewJWTVerifier 创建（service 用 ServiceVerifier、app 用 HMAC——此处拒绝）。
func NewJWTVerifier(ptype PrincipalType, opts BaseOptions) (*JWTVerifier, error) {
	if ptype == "" || ptype == PrincipalService {
		return nil, fmt.Errorf("authn: jwt verifier ptype %q invalid (service uses ServiceVerifier, app uses HMAC)", ptype)
	}
	if err := opts.validate(); err != nil {
		return nil, fmt.Errorf("authn: jwt verifier: %w", err)
	}
	return &JWTVerifier{ptype: ptype, base: opts}, nil
}

// Scheme 实现 Provider。
func (v *JWTVerifier) Scheme() string { return SchemeBearer }

// Authenticate 验证链：验签 → sub → 吊销/版本 → Principal。
func (v *JWTVerifier) Authenticate(ctx context.Context, cred Credential) (Principal, error) {
	claims, err := v.base.verify(ctx, cred)
	if err != nil {
		return Principal{}, err
	}
	id, _ := claimsString(claims, ClaimSub)
	if id == "" {
		return Principal{}, fmt.Errorf("%w: missing %s", ErrInvalidToken, ClaimSub)
	}
	jti, _ := claimsString(claims, ClaimJTI)
	tv, hasTV := claimsInt64(claims, ClaimTokenVersion)
	if err := v.base.enforce(ctx, id, jti, tv, hasTV); err != nil {
		return Principal{}, err
	}
	principal := Principal{
		Type:     v.ptype,
		ID:       id,
		Roles:    claimsStrings(claims, ClaimRoles),
		Platform: claimStringOr(claims, ClaimPlatform),
		JTI:      jti,
	}
	if exp, err := claims.GetExpirationTime(); err == nil && exp != nil {
		principal.Expires = exp.Time
	}
	if sid, ok := claimsString(claims, ClaimSessionID); ok {
		principal.SID = sid // 会话绑定 claim（identity  登出面签发）
	}
	return principal, nil
}

func claimStringOr(claims jwt.MapClaims, key string) string {
	s, _ := claimsString(claims, key)
	return s
}
