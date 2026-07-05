package jwt_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"NetyAdmin/internal/pkg/jwt"

	gwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestRSAKey 在测试中生成 2048-bit RSA 密钥对，用于 RS256 签发/解析。
// 每个用例独立生成，避免跨用例共享密钥造成干扰。
func generateTestRSAKey(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "生成 RSA 密钥对失败")
	return priv, &priv.PublicKey
}

// encodePublicKeyToPEM 将 RSA 公钥编码为 PKIX PEM 字节，用于模拟 alg confusion 攻击：
// 攻击者拿到的公钥 PEM 字节会被当作 HMAC secret 来伪造 HS256 token。
func encodePublicKeyToPEM(t *testing.T, pub *rsa.PublicKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

// TestNew_KeyAndTTLValidation 验证 New 的 fail-closed 校验：
// nil 私钥/公钥、TTL <= 0 必须返回错误。
func TestNew_KeyAndTTLValidation(t *testing.T) {
	priv, pub := generateTestRSAKey(t)

	tests := []struct {
		name       string
		privateKey *rsa.PrivateKey
		publicKey  *rsa.PublicKey
		accessTTL  time.Duration
		refreshTTL time.Duration
		wantErr    bool
		errSub     string
	}{
		{"nil private key", nil, pub, 30 * time.Minute, 168 * time.Hour, true, "私钥不能为空"},
		{"nil public key", priv, nil, 30 * time.Minute, 168 * time.Hour, true, "公钥不能为空"},
		{"zero access ttl", priv, pub, 0, 168 * time.Hour, true, "accessTokenTTL"},
		{"negative access ttl", priv, pub, -1 * time.Minute, 168 * time.Hour, true, "accessTokenTTL"},
		{"zero refresh ttl", priv, pub, 30 * time.Minute, 0, true, "refreshTokenTTL"},
		{"negative refresh ttl", priv, pub, 30 * time.Minute, -1 * time.Hour, true, "refreshTokenTTL"},
		{"valid config", priv, pub, 30 * time.Minute, 168 * time.Hour, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j, err := jwt.New(tt.privateKey, tt.publicKey, tt.accessTTL, tt.refreshTTL)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errSub != "" {
					assert.Contains(t, err.Error(), tt.errSub)
				}
				assert.Nil(t, j)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, j)
			}
		})
	}
}

// TestParseToken_RS256RoundTrip 验证 RS256 签发→解析往返流程正确。
func TestParseToken_RS256RoundTrip(t *testing.T) {
	priv, pub := generateTestRSAKey(t)
	j, err := jwt.New(priv, pub, 30*time.Minute, 168*time.Hour)
	require.NoError(t, err)

	claims := j.NewAdminClaims(1, "admin", []string{"super_admin"}, jwt.AccessToken, 1)
	token, err := j.GenerateToken(claims)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed := &jwt.AdminClaims{}
	err = j.ParseToken(token, parsed)
	assert.NoError(t, err, "RS256 签发的 token 应能被正确解析")
	assert.Equal(t, uint(1), parsed.UserID)
	assert.Equal(t, "admin", parsed.Username)
	assert.Equal(t, []string{"super_admin"}, parsed.Roles)
	assert.Equal(t, uint64(1), parsed.TokenVersion)
	assert.Equal(t, string(jwt.AccessToken), parsed.Subject)
}

// TestParseToken_AlgConfusion 验证 alg confusion 防御：
// 用 HS256 算法（以公钥 PEM 字节作为 HMAC secret）签发的伪造 token 必须被拒绝。
// 这是 RS256 迁移后最关键的防御点 —— 攻击者可能利用公开的公钥 PEM 字节
// 作为 HMAC 密钥伪造 alg=HS256 的 token，绕过签名校验。
func TestParseToken_AlgConfusion(t *testing.T) {
	priv, pub := generateTestRSAKey(t)
	j, err := jwt.New(priv, pub, 30*time.Minute, 168*time.Hour)
	require.NoError(t, err)

	// 1. 合法的 RS256 token 应解析成功
	adminClaims := j.NewAdminClaims(1, "admin", []string{"super_admin"}, jwt.AccessToken, 1)
	validToken, err := j.GenerateToken(adminClaims)
	require.NoError(t, err)
	err = j.ParseToken(validToken, &jwt.AdminClaims{})
	assert.NoError(t, err, "RS256 签名的 token 应能被正确解析")

	// 2. 构造 alg=HS256 的伪造 token：用公钥的 PEM 字节作为 HMAC secret
	//    攻击场景：公钥可公开获取，若服务端 keyFunc 不校验 alg，会把公钥字节当作 HMAC 密钥校验，
	//    导致攻击者可伪造任意 token。
	pubPEMBytes := encodePublicKeyToPEM(t, pub)
	hs256Token, err := gwt.NewWithClaims(gwt.SigningMethodHS256, adminClaims).SignedString(pubPEMBytes)
	require.NoError(t, err)

	err = j.ParseToken(hs256Token, &jwt.AdminClaims{})
	assert.Error(t, err, "HS256 签名的 token 必须被拒绝（alg confusion 防御）")
	assert.Equal(t, jwt.ErrTokenInvalid, err)

	// 3. 构造 alg=none 的伪造 token（无签名），同样应被拒绝
	noneToken := gwt.NewWithClaims(gwt.SigningMethodNone, adminClaims)
	noneStr, err := noneToken.SignedString(gwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	err = j.ParseToken(noneStr, &jwt.AdminClaims{})
	assert.Error(t, err, "alg=none 的 token 必须被拒绝")
	assert.Equal(t, jwt.ErrTokenInvalid, err)
}

// TestTTLIndependence 验证 Access / Refresh TTL 独立配置：
// access token 用 accessTokenTTL，refresh token 用 refreshTokenTTL，
// 不再是简单的 *2 关系。
func TestTTLIndependence(t *testing.T) {
	priv, pub := generateTestRSAKey(t)
	// 用易区分的 TTL：access = 1 分钟，refresh = 7 天
	accessTTL := 1 * time.Minute
	refreshTTL := 168 * time.Hour
	j, err := jwt.New(priv, pub, accessTTL, refreshTTL)
	require.NoError(t, err)

	now := time.Now()
	accessClaims := j.NewAdminClaims(1, "admin", []string{"admin"}, jwt.AccessToken, 1)
	refreshClaims := j.NewAdminClaims(1, "admin", []string{"admin"}, jwt.RefreshToken, 1)

	accessExp := accessClaims.ExpiresAt.Time
	refreshExp := refreshClaims.ExpiresAt.Time

	// access 过期时间应在 [now+accessTTL, now+accessTTL+600s] 范围内（含 jitter）
	accessLower := now.Add(accessTTL)
	accessUpper := now.Add(accessTTL + 600*time.Second)
	assert.True(t, accessExp.After(accessLower) || accessExp.Equal(accessLower),
		"access exp %s 应 >= %s", accessExp, accessLower)
	assert.True(t, accessExp.Before(accessUpper) || accessExp.Equal(accessUpper),
		"access exp %s 应 <= %s", accessExp, accessUpper)

	// refresh 过期时间应在 [now+refreshTTL, now+refreshTTL+600s] 范围内
	refreshLower := now.Add(refreshTTL)
	refreshUpper := now.Add(refreshTTL + 600*time.Second)
	assert.True(t, refreshExp.After(refreshLower) || refreshExp.Equal(refreshLower),
		"refresh exp %s 应 >= %s", refreshExp, refreshLower)
	assert.True(t, refreshExp.Before(refreshUpper) || refreshExp.Equal(refreshUpper),
		"refresh exp %s 应 <= %s", refreshExp, refreshUpper)

	// access 与 refresh 过期时间差应远大于 accessTTL（验证不是 *2 关系）
	diff := refreshExp.Sub(accessExp)
	assert.True(t, diff > 100*time.Hour,
		"refresh exp 应远大于 access exp（独立 TTL，非 *2 关系），实际差值 %s", diff)
}

// TestTokenVersion_RoundTrip 验证 TokenVersion 字段在签发→解析往返中正确保留（BUG #5 回归测试）。
func TestTokenVersion_RoundTrip(t *testing.T) {
	priv, pub := generateTestRSAKey(t)
	j, err := jwt.New(priv, pub, 30*time.Minute, 168*time.Hour)
	require.NoError(t, err)

	tests := []struct {
		name    string
		version uint64
	}{
		{"zero", 0},
		{"one", 1},
		{"large", 1<<32 + 12345},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := j.NewAdminClaims(42, "tester", []string{"admin"}, jwt.AccessToken, tt.version)
			token, err := j.GenerateToken(claims)
			assert.NoError(t, err)

			parsed := &jwt.AdminClaims{}
			err = j.ParseToken(token, parsed)
			assert.NoError(t, err)
			assert.Equal(t, tt.version, parsed.TokenVersion, "TokenVersion 应在往返中保留")
			assert.Equal(t, uint(42), parsed.UserID)
		})
	}
}

// TestUserClaims_RoundTrip 验证 UserClaims（C 端）RS256 往返正确。
func TestUserClaims_RoundTrip(t *testing.T) {
	priv, pub := generateTestRSAKey(t)
	j, err := jwt.New(priv, pub, 30*time.Minute, 168*time.Hour)
	require.NoError(t, err)

	claims := j.NewUserClaims("user-123", "web", jwt.AccessToken, 5)
	token, err := j.GenerateToken(claims)
	require.NoError(t, err)

	parsed := &jwt.UserClaims{}
	err = j.ParseToken(token, parsed)
	assert.NoError(t, err)
	assert.Equal(t, "user-123", parsed.UID)
	assert.Equal(t, "web", parsed.Platform)
	assert.Equal(t, uint64(5), parsed.TokenVersion)
	assert.Equal(t, "user", parsed.Type)
}
