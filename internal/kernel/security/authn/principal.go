// principal.go：统一身份模型——Principal 四类（admin/user/service/app）、
// ctx 注入/提取、跨子系统传播格式（Bus Headers.principal）。
// 错误哨兵与状态码映射（10101-10104）由调用方完成；authn 只返回领域错误。
package authn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PrincipalType 是身份类别（§8.1 Provider 表）。
type PrincipalType string

const (
	PrincipalAdmin   PrincipalType = "admin"   // 管理端（ID-01 Admin JWT）
	PrincipalUser    PrincipalType = "user"    // C 端（US-01 users 模块自注册）
	PrincipalService PrincipalType = "service" // 内部 RPC（TR-02 Service JWT）
	PrincipalApp     PrincipalType = "app"     // 外部应用（App HMAC）
)

// Principal 是验证通过后的统一身份。
// 不携带 token 原文；exp 已在验证链内判定，验证通过即存活。
type Principal struct {
	Type     PrincipalType
	ID       string    // admin/user：ULID；service/app：具名 ID
	Roles    []string  // admin：角色（RB-01 的 AuthZ 判定输入）
	Platform string    // user：客户端平台（web/app/mini…，使用方定义枚举）
	Scopes   []string  // service/app：允许的 port.method或 API 白名单
	Act      string    // service：代行的原始 principal
	JTI      string    // token id：吊销面判定键（IsRevoked）
	SID      string    // 会话绑定（access 的 sid claim；登出经 sid 杀 refresh 链——）
	Expires  time.Time // access 过期时刻（登出 TTL 计算；零值=非 JWT 面）
}

// String 是传播格式：`type:id`（Bus Headers.principal）。
// 有损：Roles/Scopes/Act 不进消息头（消息头是传播上下文不是凭证，消费方
// 需要全量身份时从原始 claim/会话取，避免伪造面扩大）。
func (p Principal) String() string { return string(p.Type) + ":" + p.ID }

// —— 领域错误（调用方映射状态码；不携带底层细节） ——

var (
	// ErrNoCredential：请求未携带凭证（10101）。
	ErrNoCredential = errors.New("authn: no credential")
	// ErrInvalidToken：验签/格式/claim 契约失败（10102）。
	ErrInvalidToken = errors.New("authn: token invalid")
	// ErrExpiredToken：过期（10103；errors.Is 同时命中 ErrInvalidToken）。
	ErrExpiredToken = fmt.Errorf("%w: expired", ErrInvalidToken)
	// ErrRevokedToken：jti 已吊销或 TokenVersion 失配——两者都是「token 已作废」
	// （10104；用户级全作废复用同一语义，不新增码）。
	ErrRevokedToken = errors.New("authn: token revoked")
	// ErrUnknownScheme：请求的认证方案未注册（装配错误面）。
	ErrUnknownScheme = errors.New("authn: unknown scheme")
)

// Redis 不可用等系统错误不定义哨兵：session.ErrUnavailable 按错误链透传
// （multi fail_closed / single db 回退语义已由 session 包保证），调用方
// errors.Is 判定后映射 10010。

// —— ctx 传播（§11 上下文传播规则：principal 全链路传递） ——

type ctxKey struct{}

// WithPrincipal 将身份注入 ctx。
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// PrincipalFrom 取 ctx 身份（匿名请求 ok=false——匿名是合法状态，
// 接入面按路由元数据决定是否要求认证）。
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// ParseString 解析传播格式（type:id）——Bus 消费侧回注 ctx 用（与 String
// 有损对称：只还原 Type/ID；Roles/Scopes/Act 不在消息头，需全量身份时
// 从原始 claim 取）。格式坏返回 false（传播上下文缺失不阻断消息处理）。
func ParseString(s string) (Principal, bool) {
	t, id, ok := strings.Cut(s, ":")
	if !ok || t == "" || id == "" {
		return Principal{}, false
	}
	return Principal{Type: PrincipalType(t), ID: id}, true
}
