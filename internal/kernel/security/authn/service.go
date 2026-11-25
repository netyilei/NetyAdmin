// service.go：内部 RPC 服务 JWT 验证——
//
//	sub = 调用方服务名；scp = 允许的 port.method 白名单；act = 代行的原始
//	principal；exp ≤ MaxLifetime（默认 60s，验证方强制短寿命——签发契约的
//	接收侧对偶校验，纵深防御）；trace_id 走头传播不进 claim（TR-02 结论——
//	豁免留痕，claim 面随首个跨进程 span 续链需求重估）。
//
// 接收方端口级校验（scope 覆盖判定、敏感操作对 act 二次 AuthZ）在 TR-02/
// 编排；本包只负责把验证通过的 service 身份交给调用方。
package authn

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// defaultServiceLifetime 是的 exp≤60s 签发契约（验证方默认强制同值）。
const defaultServiceLifetime = 60 * time.Second

// ServiceOptions 是服务 JWT 验证参数。
type ServiceOptions struct {
	BaseOptions
	// MaxLifetime 是 exp-iat 上限（<=0 取默认 60s；签发契约的验证方强制）。
	MaxLifetime time.Duration
	// OptionalScopes 放宽 scp 必填（默认必须存在且非空——契约）。
	OptionalScopes bool
}

// ServiceVerifier 是内部 RPC 服务 JWT Provider（TR-02 消费）。
type ServiceVerifier struct {
	base           BaseOptions
	lifetime       time.Duration
	optionalScopes bool
}

// NewServiceVerifier 创建。
func NewServiceVerifier(opts ServiceOptions) (*ServiceVerifier, error) {
	if err := opts.BaseOptions.validate(); err != nil {
		return nil, fmt.Errorf("authn: service verifier: %w", err)
	}
	lifetime := opts.MaxLifetime
	if lifetime <= 0 {
		lifetime = defaultServiceLifetime
	}
	return &ServiceVerifier{base: opts.BaseOptions, lifetime: lifetime, optionalScopes: opts.OptionalScopes}, nil
}

// Scheme 实现 Provider。
func (v *ServiceVerifier) Scheme() string { return SchemeBearer }

// Authenticate 验证链：验签 → sub → 短寿命 → scp → 吊销/版本 → Principal。
func (v *ServiceVerifier) Authenticate(ctx context.Context, cred Credential) (Principal, error) {
	claims, err := v.base.verify(ctx, cred)
	if err != nil {
		return Principal{}, err
	}
	id, _ := claimsString(claims, ClaimSub)
	if id == "" {
		return Principal{}, fmt.Errorf("%w: missing %s", ErrInvalidToken, ClaimSub)
	}
	if err := v.checkLifetime(claims); err != nil {
		return Principal{}, err
	}
	scopes := claimsStrings(claims, ClaimScopes)
	if !v.optionalScopes && len(scopes) == 0 {
		return Principal{}, fmt.Errorf("%w: missing/empty %s", ErrInvalidToken, ClaimScopes)
	}
	jti, _ := claimsString(claims, ClaimJTI)
	tv, hasTV := claimsInt64(claims, ClaimTokenVersion)
	if err := v.base.enforce(ctx, id, jti, tv, hasTV); err != nil {
		return Principal{}, err
	}
	return Principal{
		Type:   PrincipalService,
		ID:     id,
		Scopes: scopes,
		Act:    claimStringOr(claims, ClaimAct),
		JTI:    jti,
	}, nil
}

// checkLifetime 强制 exp-iat ≤ MaxLifetime（iat 缺失 = 签发契约违规 → 拒绝）。
func (v *ServiceVerifier) checkLifetime(claims jwt.MapClaims) error {
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return fmt.Errorf("%w: missing exp", ErrInvalidToken) // primitives 已强制；双保险
	}
	iat, err := claims.GetIssuedAt()
	if err != nil || iat == nil {
		return fmt.Errorf("%w: missing iat", ErrInvalidToken)
	}
	if exp.Time.Sub(iat.Time) > v.lifetime {
		return fmt.Errorf("%w: lifetime %s exceeds cap %s", ErrInvalidToken, exp.Time.Sub(iat.Time), v.lifetime)
	}
	return nil
}
