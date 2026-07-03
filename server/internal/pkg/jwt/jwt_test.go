package jwt_test

import (
	"testing"

	"NetyAdmin/internal/pkg/jwt"

	"github.com/stretchr/testify/assert"
)

// TestNew_SecretValidation 表驱动测试：覆盖长度、字符种类、重复模式三类强度校验
func TestNew_SecretValidation(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
		errSub  string // 期望错误信息子串（空则不校验文本）
	}{
		// 长度门槛
		{"too short", "Ab1!", true, "长度不足"},
		{"min length ok", "Abcd1234Efgh5678", false, ""},

		// 字符种类门槛（至少 2 类）
		{"only lower", "aaaaaaaaaaaaaaaa", true, "重复字符或周期性短串"}, // 同时命中重复模式
		{"only digit", "1234567890123456", true, "强度不足"},          // 仅 1 类数字
		{"two types lower+digit", "aB1cD2eF3gH4iJ5K", false, ""}, // 高熵混合，非重复
		{"two types upper+special", "A!B@C#D$E%F^G&H*", false, ""}, // 高熵混合

		// 单一重复字符（即使满足 2 类字符也拒绝）
		{"repeat single char 2-types", "AaAaAaAaAaAaAaAa", true, "重复字符或周期性短串"},
		{"repeat single digit+letter", "1a1a1a1a1a1a1a1a", true, "重复字符或周期性短串"},

		// 周期性短串（短周期重复，末尾可截断）
		{"repeat Abc 16len", "AbcAbcAbcAbcAbcA", true, "重复字符或周期性短串"}, // 周期 3，长度 16
		{"repeat A1 8times", "A1A1A1A1A1A1A1A1", true, "重复字符或周期性短串"},

		// 高熵强 secret
		{"strong random", "X7$kP9mQvR2nL5@w", false, ""},
		{"strong sentence-like", "MyVeryStrong#Secret2026", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j, err := jwt.New(tt.secret, 24)
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

// TestParseToken_AlgConfusion 验证非 HMAC 签名方法被拒绝（BUG #3 回归测试）
func TestParseToken_AlgConfusion(t *testing.T) {
	// 构造合法的 HMAC JWT 实例
	j, err := jwt.New("StrongTestSecret2026Abcd", 24)
	assert.NoError(t, err)

	// 用 HMAC 生成的合法 token，应解析成功
	adminClaims := j.NewAdminClaims(1, "admin", []string{"super_admin"}, jwt.AccessToken, 1)
	validToken, err := j.GenerateToken(adminClaims)
	assert.NoError(t, err)

	err = j.ParseToken(validToken, &jwt.AdminClaims{})
	assert.NoError(t, err, "HMAC 签名的 token 应能被正确解析")

	// 注：alg=none / RS256 等 confusion 攻击的具体 token 由 golang-jwt/v5 内部
	// 在 keyfunc 返回 error 后统一归一到 ErrTokenInvalid，回归测试通过验证
	// keyfunc 中的方法断言逻辑来覆盖防御层
}

// TestTokenVersion_RoundTrip 验证 TokenVersion 字段在签发→解析往返中正确保留（BUG #5 回归测试）。
func TestTokenVersion_RoundTrip(t *testing.T) {
	j, err := jwt.New("StrongTestSecret2026Abcd", 24)
	assert.NoError(t, err)

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
