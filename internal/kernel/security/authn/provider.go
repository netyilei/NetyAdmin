// provider.go：Provider 化认证入口——凭证载体、Provider 契约、
// 按 scheme 分发的 Chain。接入面按需组合 Provider（adminapi/clientapi/gateway
// 各自装配所需方案），不再有两套平行鉴权链。
package authn

import (
	"context"
	"fmt"
	"sync"
)

// Credential 是请求侧凭证载体（transport 中间件从 HTTP 头翻译而来；
// provider 各取所需：JWT 只用 Token，HMAC 用 Method/Path/Headers）。
type Credential struct {
	Token   string            // Bearer 凭证 / HMAC 签名值
	Method  string            // HMAC：请求方法（签名串输入）
	Path    string            // HMAC：请求路径（签名串输入）
	Payload string            // HMAC：签名串第五段（定稿：GET=query 排序串/非 GET=body sha256 hex——edge 层构造）
	Headers map[string]string // HMAC：app-id/timestamp/nonce/body-sha256
}

// Scheme 常量（§8.1；Chain 按 scheme 分发，一个接入面 Chain 每方案一个实例）。
const (
	SchemeBearer = "bearer" // Admin/User/Service JWT
	SchemeHMAC   = "hmac"   // App HMAC
)

// Provider 是一种认证方案。
type Provider interface {
	// Scheme 是凭证方案标识（"bearer" | "hmac"；接入面按 scheme 分发）。
	Scheme() string
	// Authenticate 验证凭证并返回 Principal；失败返回领域错误
	// （ErrNoCredential/ErrInvalidToken/ErrExpiredToken/ErrRevokedToken），
	// 系统错误（吊销面 Redis 不可用等）原样透传。
	Authenticate(ctx context.Context, cred Credential) (Principal, error)
}

// Chain 是 Provider 注册表（scheme 唯一；注册期去重，运行期只读分发）。
type Chain struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewChain 创建空 Chain。
func NewChain() *Chain {
	return &Chain{providers: map[string]Provider{}}
}

// Register 注册 Provider（scheme 重复 → 错误；同一 scheme 只允许一种实现）。
func (c *Chain) Register(p Provider) error {
	if p == nil {
		return fmt.Errorf("authn: nil provider")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, dup := c.providers[p.Scheme()]; dup {
		return fmt.Errorf("authn: duplicate scheme %q", p.Scheme())
	}
	c.providers[p.Scheme()] = p
	return nil
}

// Authenticate 按 scheme 分发（未注册 scheme → ErrUnknownScheme）。
func (c *Chain) Authenticate(ctx context.Context, scheme string, cred Credential) (Principal, error) {
	c.mu.RLock()
	p, ok := c.providers[scheme]
	c.mu.RUnlock()
	if !ok {
		return Principal{}, ErrUnknownScheme
	}
	return p.Authenticate(ctx, cred)
}
