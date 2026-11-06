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
