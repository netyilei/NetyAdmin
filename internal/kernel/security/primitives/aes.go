// aes.go：AES-256-GCM 加解密原语（App Secret 落库加密；V1 utils/crypto
// 同语义——12 字节随机 nonce 前缀 + GCM Seal/Open；key 32 字节由 Secret 注入
// （config secrets 段 env-only—— 分层）。
package primitives

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// ErrAESKeySize 是 key 非 32 字节的错误（AES-256 固定——V1 允容 16，
// V2 收紧 256 位）。
var ErrAESKeySize = errors.New("primitives: aes key must be 32 bytes (AES-256)")

// EncryptAESGCM 加密（base64(nonce||ciphertext)；key 必须 32 字节）。
func EncryptAESGCM(key []byte, plaintext []byte) (string, error) {
	if len(key) != 32 {
		return "", ErrAESKeySize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("primitives: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("primitives: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("primitives: nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// RandomToken 生成 base64url 随机 token（n 字节熵 → ≈4/3n 字符；AppSecret
// 重置等一次性凭证生成用——G1-06 收拢点）。
func RandomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("primitives: random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// DecryptAESGCM 解密（base64(nonce||ciphertext) 反向）。
func DecryptAESGCM(key []byte, encoded string) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrAESKeySize
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("primitives: aes decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("primitives: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("primitives: gcm: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("primitives: ciphertext too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("primitives: gcm open: %w", err)
	}
	return plain, nil
}
