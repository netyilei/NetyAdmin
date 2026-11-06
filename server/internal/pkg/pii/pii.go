// Package pii 提供手机号类个人敏感信息（PII）的静态加密单一事实源：
// 归一化、AES-GCM 随机加密（存储列）、HMAC-SHA256 盲索引（检索列）、
// 尾号检索（明文辅助列）与展示掩码。
//
// 设计（docs/CHANGELOG.md 2026-11-07 条目）：
//   - 存储密文（随机化，抗频率分析）、检索走盲索引、出口掩码；
//   - enc/idx 双子密钥由 HKDF-SHA256 从 [security].aes_key 派生，
//     不新增配置项且用途隔离（GCM 加密与 HMAC 检索分域）；
//   - 中缀模糊搜索在加密下无法安全保留（数学代价），检索语义 =
//     完整号精确（盲索引）+ 尾号 4 位精确（明文辅助列）。
//
// 归一化必须在加密与索引前执行：同一号码的 +86 前缀/空格/连字符多形态
// 必须映射到同一密文与同一盲索引，否则唯一索引可被形态差异绕过。
package pii

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// ErrInvalidPhone 是手机号格式非法的错误（写入口校验用，^1[3-9]\d{9}$）。
var ErrInvalidPhone = errors.New("pii: invalid phone format")

// hkdfSalt 与 info 将主密钥按用途分离：enc（AES-GCM 存储密钥）与
// idx（HMAC-SHA256 盲索引密钥）永不共用，防跨用途密钥重放。
var (
	hkdfSalt = []byte("netyadmin/pii/v1")
	encInfo  = []byte("netyadmin/pii/v1/phone-enc")
	idxInfo  = []byte("netyadmin/pii/v1/phone-idx")
)

// Cipher 是手机号 PII 加密器。构造后并发安全（密钥只读）。
type Cipher struct {
	encKey []byte
	idxKey []byte
}

// NewCipher 从主密钥（[security].aes_key）派生子密钥构造 Cipher。
// 主密钥为空时派生仍可运行（开发兜底），但生产环境由 ValidateConfig
// 强制 aes_key 非默认值，不依赖本包兜底。
func NewCipher(masterKey string) *Cipher {
	return &Cipher{
		encKey: derive(masterKey, encInfo),
		idxKey: derive(masterKey, idxInfo),
	}
}

// derive 用 HKDF-SHA256 从主密钥派生 32 字节子密钥。HKDF 接受任意长度输入，
// 输出恒为 32 字节（AES-256 / HMAC-SHA256 密钥长度）。
func derive(master string, info []byte) []byte {
	out := make([]byte, 32)
	r := hkdf.New(sha256.New, []byte(master), hkdfSalt, info)
	_, _ = io.ReadFull(r, out) // crypto/rand 源不会失败
	return out
}

// Normalize 归一化手机号：去首尾空格、去内部空格/连字符、剥离国际前缀
// （+86 / 86）。不做格式强校验（脏数据也必须能算索引、能迁移）。
func Normalize(phone string) string {
	p := strings.TrimSpace(phone)
	p = strings.ReplaceAll(p, " ", "")
	p = strings.ReplaceAll(p, "-", "")
	for _, prefix := range []string{"+86", "86"} {
		if strings.HasPrefix(p, prefix) && len(p) > len(prefix) {
			p = p[len(prefix):]
			break
		}
	}
	return p
}

// Validate 校验归一化后的手机号为中国大陆 11 位手机号（写入口用）。
func Validate(phone string) error {
	p := Normalize(phone)
	if len(p) != 11 || p[0] != '1' || p[1] < '3' || p[1] > '9' {
		return fmt.Errorf("%w: %q", ErrInvalidPhone, phone)
	}
	for i := 0; i < 11; i++ {
		if p[i] < '0' || p[i] > '9' {
			return fmt.Errorf("%w: %q", ErrInvalidPhone, phone)
		}
	}
	return nil
}

// Seal 一次性产出手机号的三个持久化形态：
//   - enc:    AES-256-GCM 随机密文（base64），存储列；
//   - idx:    HMAC-SHA256 盲索引（hex），精确匹配/唯一索引列；
//   - tail4:  归一化后末 4 位（不足 4 位取全串），明文辅助检索列。
//
// 同一号码多次 Seal 的 enc 互不相同（随机 nonce），idx 恒相同。
func (c *Cipher) Seal(phone string) (enc, idx, tail4 string, err error) {
	p := Normalize(phone)
	if p == "" {
		return "", "", "", nil
	}
	enc, err = c.sealGCM(p)
	if err != nil {
		return "", "", "", err
	}
	return enc, c.Index(p), tail4Of(p), nil
}

// Open 解密存储密文，返回明文。密文为空返回空串（未设置手机号的行）。
// 解密失败（密钥不符/密文损坏）返回错误——调用方应记日志并按空号降级，
// 不得让单行脏数据阻断列表查询。
func (c *Cipher) Open(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", fmt.Errorf("pii: decode base64: %w", err)
	}
	block, err := aes.NewCipher(c.encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("pii: ciphertext shorter than nonce")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("pii: gcm open: %w", err)
	}
	return string(plain), nil
}

// Index 返回归一化手机号的 HMAC-SHA256 盲索引（hex）。空号返回空串。
func (c *Cipher) Index(phone string) string {
	p := Normalize(phone)
	if p == "" {
		return ""
	}
	m := hmac.New(sha256.New, c.idxKey)
	m.Write([]byte(p))
	return hex.EncodeToString(m.Sum(nil))
}

// sealGCM 与 utils.Encrypt 同为 AES-GCM，但密钥独立派生（用途隔离），
// 且返回裸 base64 密文（nonce 前置）。
func (c *Cipher) sealGCM(plaintext string) (string, error) {
	block, err := aes.NewCipher(c.encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// tail4Of 取归一化号码末 4 位；不足 4 位取全串。
func tail4Of(p string) string {
	if len(p) <= 4 {
		return p
	}
	return p[len(p)-4:]
}

// Mask 返回展示掩码：保留前 3 后 4（138****5678）。
// 非 11 位（含空串/短号/已掩码值）原样返回，避免二次掩码或越界。
func Mask(phone string) string {
	p := Normalize(phone)
	if len(p) != 11 {
		return phone
	}
	return p[:3] + "****" + p[len(p)-4:]
}

// IsMasked 判断值是否已是掩码形态（含 '*'）。
// 用于编辑回写防御：前端把掩码值原样提交时不当作"改号"处理。
func IsMasked(phone string) bool {
	return strings.Contains(phone, "*")
}
