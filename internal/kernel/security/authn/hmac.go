// hmac.go：外部 App HMAC 验证（App principal； HMAC 原语）。
//
// 分工边界：verifier 负责 app 密钥解析 + 签名串验证 + timestamp 时间窗；
// nonce 防重放占用由编排（coordination nonce）；App 表/密钥来源
// 由注入。签名串契约（canonical，\n 连接）：
//
//	method \n path \n timestamp \n nonce \n body_sha256_hex
//
// canonical 细节随与 V1 OpenPlatformAuth 对照后定稿校准——当前为
// 最小明确版本（登记）。
package authn

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
)

// HMAC 凭证约定头（小写；transport/ 翻译到 Credential.Headers）。
const (
	HeaderAppID     = "x-na-app-id"
	HeaderTimestamp = "x-na-timestamp"
	HeaderNonce     = "x-na-nonce"
)

// defaultHMACSkew 是 timestamp 允许偏差（±60s—— 对照 V1 定稿；
// nonce TTL 2 分钟覆盖整个窗口——重放主防线是 nonce 占用）。
const defaultHMACSkew = 60 * time.Second

// HMACOptions 是 App HMAC 验证参数。
type HMACOptions struct {
	// Secret 按 appID 取密钥；未知 app 返回错误（fail-closed 原样透传拒绝）。
	Secret  func(ctx context.Context, appID string) ([]byte, error)
	Clock   primitives.Clock // 时间窗判定（nil = SystemClock）
	MaxSkew time.Duration    // <=0 取默认 60s（defaultHMACSkew）
}

// AppHMACVerifier 是外部 App HMAC Provider（消费）。
type AppHMACVerifier struct {
	scheme string
	opts   HMACOptions
}

// NewAppHMACVerifier 创建（Secret 必填）。
func NewAppHMACVerifier(opts HMACOptions) (*AppHMACVerifier, error) {
	if opts.Secret == nil {
		return nil, errors.New("authn: hmac verifier requires Secret")
	}
	if opts.Clock == nil {
		opts.Clock = primitives.SystemClock{}
	}
	if opts.MaxSkew <= 0 {
		opts.MaxSkew = defaultHMACSkew
	}
	return &AppHMACVerifier{scheme: SchemeHMAC, opts: opts}, nil
}

// Scheme 实现 Provider。
func (v *AppHMACVerifier) Scheme() string { return v.scheme }

// Authenticate 验证链：签名要素 → 时间窗 → 密钥 → 签名串 → Principal。
// Principal 无 JTI（per-request 签名无吊销面；重放防护在 nonce 编排）；
// Scopes 由 edge 层在 AppAuthSnapshot 取得后填充（兑现——§8.1
// App 行 Scopes 填充销项）。
func (v *AppHMACVerifier) Authenticate(ctx context.Context, cred Credential) (Principal, error) {
	appID := cred.Headers[HeaderAppID]
	ts := cred.Headers[HeaderTimestamp]
	nonce := cred.Headers[HeaderNonce]
	// payload 允许空串（GET 无 query/空 body——canonical 尾段空，V1 同语义；
	// 评审修：此前空 payload 被判缺凭证=所有 GET 无参请求 401）
	if cred.Token == "" || cred.Method == "" || cred.Path == "" ||
		appID == "" || ts == "" || nonce == "" {
		return Principal{}, ErrNoCredential
	}
	tsUnix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: bad timestamp", ErrInvalidToken)
	}
	// 溢出防御（审核 F4：极值 ts 经 Duration 乘法可回绕绕窗——先以秒判定）
	if diffSec := v.opts.Clock.Now().Unix() - tsUnix; diffSec > int64(v.opts.MaxSkew/time.Second) ||
		diffSec < -int64(v.opts.MaxSkew/time.Second) {
		return Principal{}, fmt.Errorf("%w: timestamp skew %ds > %s", ErrExpiredToken, diffSec, v.opts.MaxSkew)
	}
	key, err := v.opts.Secret(ctx, appID)
	if err != nil {
		return Principal{}, fmt.Errorf("authn: app secret: %w", err)
	}
	// canonical 第五段=payload（edge 构造—— 定稿）；method 大写归一（V1 同构）
	method := strings.ToUpper(cred.Method)
	msg := method + "\n" + cred.Path + "\n" + ts + "\n" + nonce + "\n" + cred.Payload
	if !primitives.VerifyHMAC(key, []byte(msg), cred.Token) {
		return Principal{}, ErrInvalidToken
	}
	return Principal{Type: PrincipalApp, ID: appID}, nil
}
