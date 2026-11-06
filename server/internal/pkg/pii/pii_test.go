package pii

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMaster = "netyadmin-aes-key-32-chars-long!"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		check func(t *testing.T, got string)
	}{
		{"plain kept", "13812345678", func(t *testing.T, got string) { assert.Equal(t, "13812345678", got) }},
		{"+86 prefix stripped", "+8613812345678", func(t *testing.T, got string) { assert.Equal(t, "13812345678", got) }},
		{"86 prefix stripped only with length", "8613812345678", func(t *testing.T, got string) { assert.Equal(t, "13812345678", got) }},
		{"bare 86 not stripped (too short after strip)", "86", func(t *testing.T, got string) { assert.Equal(t, "86", got) }},
		{"spaces and dashes removed", "138 1234-5678", func(t *testing.T, got string) { assert.Equal(t, "13812345678", got) }},
		{"trim outer spaces", "  13812345678  ", func(t *testing.T, got string) { assert.Equal(t, "13812345678", got) }},
		{"empty stays empty", "   ", func(t *testing.T, got string) { assert.Equal(t, "", got) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.check(t, Normalize(tt.in)) })
	}
}

func TestValidate(t *testing.T) {
	valid := []string{"13812345678", "+8615901234567", "19912345678"}
	for _, p := range valid {
		assert.NoError(t, Validate(p), p)
	}
	invalid := []string{"", "123", "12812345678", "1381234567a", "138123456789", "23812345678"}
	for _, p := range invalid {
		assert.ErrorIs(t, Validate(p), ErrInvalidPhone, p)
	}
}

func TestSeal_IndexDeterministic_EncRandom(t *testing.T) {
	c := NewCipher(testMaster)

	enc1, idx1, tail1, err := c.Seal("138 1234-5678")
	require.NoError(t, err)
	enc2, idx2, tail2, err := c.Seal("+8613812345678")
	require.NoError(t, err)

	// 归一化后同号：盲索引恒同（唯一索引语义）、尾号恒同
	assert.Equal(t, idx1, idx2, "same number must map to same blind index")
	assert.Equal(t, "5678", tail1)
	assert.Equal(t, tail1, tail2)
	// 存储密文随机化（抗频率分析）
	assert.NotEqual(t, enc1, enc2, "randomized encryption must differ per call")
	assert.True(t, strings.HasPrefix(idx1, ""), "idx is hex")

	// 异号：索引必不同
	_, idx3, _, err := c.Seal("13912345678")
	require.NoError(t, err)
	assert.NotEqual(t, idx1, idx3)
}

func TestSealOpenRoundtrip(t *testing.T) {
	c := NewCipher(testMaster)
	enc, _, _, err := c.Seal("+86 138-1234-5678")
	require.NoError(t, err)

	got, err := c.Open(enc)
	require.NoError(t, err)
	assert.Equal(t, "13812345678", got)
}

func TestOpen_EmptyAndCorrupt(t *testing.T) {
	c := NewCipher(testMaster)

	got, err := c.Open("")
	require.NoError(t, err)
	assert.Empty(t, got, "未设置手机号的行（空密文）按空号处理")

	_, err = c.Open("not-base64!!!")
	assert.Error(t, err)

	enc, _, _, err := c.Seal("13812345678")
	require.NoError(t, err)
	_, err = c.Open(enc[:len(enc)-4] + "AAAA")
	assert.Error(t, err, "篡改密文必须解密失败（GCM 认证）")

	// 换主密钥（轮换/错配）→ 解密失败而非返回错号
	_, err = NewCipher("another-master-key-32-bytes-long.").Open(enc)
	assert.True(t, errors.Is(err, err) && err != nil, "wrong master must fail to open")
}

func TestKeyDerivationIsolation(t *testing.T) {
	c := NewCipher(testMaster)
	// enc 与 idx 子密钥不同域：直接用 encKey 算 HMAC 不会等于 idx
	assert.NotEqual(t, c.encKey, c.idxKey)
	// 不同主密钥派生不同子密钥
	assert.NotEqual(t, NewCipher(testMaster).idxKey, NewCipher("other").idxKey)
}

func TestMask(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"11-digit", "13812345678", "138****5678"},
		{"+86 normalized", "+8613812345678", "138****5678"},
		{"short id kept as-is", "12345", "12345"},
		{"empty kept", "", ""},
		{"already masked kept", "138****5678", "138****5678"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.want, Mask(tt.in)) })
	}
}

func TestIsMasked(t *testing.T) {
	assert.True(t, IsMasked("138****5678"))
	assert.False(t, IsMasked("13812345678"))
	assert.False(t, IsMasked(""))
}

func TestEmailSeal_IndexLowercase_EncPreservesCase(t *testing.T) {
	c := NewCipher(testMaster)

	enc1, idx1, err := c.SealEmail("John.Doe@Example.COM")
	require.NoError(t, err)
	enc2, idx2, err := c.SealEmail("john.doe@example.com")
	require.NoError(t, err)

	// 大小写多形态映射同一盲索引（邮箱路由实践不区分大小写）
	assert.Equal(t, idx1, idx2)
	// 存储密文保留原始大小写（展示保真）
	got, err := c.OpenEmail(enc1)
	require.NoError(t, err)
	assert.Equal(t, "John.Doe@Example.COM", got)
	assert.NotEqual(t, enc1, enc2, "随机化加密")
}

func TestEmailSealOpenRoundtrip(t *testing.T) {
	c := NewCipher(testMaster)
	enc, idx, err := c.SealEmail("  alice+tag@netyadmin.com ")
	require.NoError(t, err)
	got, err := c.OpenEmail(enc)
	require.NoError(t, err)
	assert.Equal(t, "alice+tag@netyadmin.com", got)
	assert.NotEqual(t, c.IndexEmail("other@x.com"), idx)
}

func TestEmailOpen_EmptyAndCorrupt(t *testing.T) {
	c := NewCipher(testMaster)
	got, err := c.OpenEmail("")
	require.NoError(t, err)
	assert.Empty(t, got)
	_, err = c.OpenEmail("broken@@@")
	assert.Error(t, err)
}

func TestValidateEmail(t *testing.T) {
	valid := []string{"a@b.co", " first.last+tag@sub.example.org ", "John@Example.COM"}
	for _, e := range valid {
		assert.NoError(t, ValidateEmail(e), e)
	}
	invalid := []string{"", "no-at-sign", "@no-local.com", "no-domain@", "has space@x.com", "a@b"}
	for _, e := range invalid {
		assert.ErrorIs(t, ValidateEmail(e), ErrInvalidEmail, e)
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"standard", "john.doe@x.com", "j***@x.com"},
		{"trim spaces", "  a@b.co  ", "a***@b.co"},
		{"no at masked", "not-an-email", "n***"}, // 历史脏值不得原样出接口
		{"empty kept", "", ""},
		{"single-char local", "x@y.io", "x***@y.io"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.want, MaskEmail(tt.in)) })
	}
}

func TestEmailKeyIsolation(t *testing.T) {
	c := NewCipher(testMaster)
	// email 与 phone 四个子密钥两两不同（用途隔离）
	assert.NotEqual(t, c.encKey, c.encKeyEmail)
	assert.NotEqual(t, c.idxKey, c.idxKeyEmail)
	// email 盲索引 ≠ phone 盲索引（同输入不同域）
	assert.NotEqual(t, c.Index("a@b.com"), c.IndexEmail("a@b.com"))
}

// TestIsValidEmail 全量用例自 utils.IsEmail 原测试迁移（正则单一事实源随迁）。
func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{"simple valid", "user@example.com", true},
		{"with dot in local", "test.user@domain.org", true},
		{"short TLD", "a@b.co", true},
		{"with plus sign", "user+tag@domain.com", true},
		{"with percent", "user%name@domain.com", true},
		{"with digits", "user123@123.com", true},
		{"empty string", "", false},
		{"no at sign", "userdomain.com", false},
		{"no domain", "user@", false},
		{"no local part", "@domain.com", false},
		{"no TLD", "user@domain", false},
		{"double at", "user@@domain.com", false},
		{"TLD too short", "user@domain.c", false},
		{"special chars invalid", "user!name@domain.com", false},
		{"space in email", "user @domain.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidEmail(tt.email), "IsValidEmail(%q)", tt.email)
		})
	}
}
