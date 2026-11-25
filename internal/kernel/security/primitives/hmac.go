package primitives

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrHMACMismatch 是 HMAC 验签失败的领域错误（调用方映射接口状态码）。
var ErrHMACMismatch = errors.New("primitives: hmac mismatch")

// SignHMAC 计算 HMAC-SHA256 的 hex 编码（storage 上传凭证/网关签名共用原语）。
func SignHMAC(key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC 恒定时间比较签名（防时序侧信道）；签名大小写不敏感（hex 规范化）。
func VerifyHMAC(key, data []byte, sig string) bool {
	expected := SignHMAC(key, data)
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(sig)))
}

// NewNonce 生成 n 字节随机数的 hex 编码（security.md：nonce 原语；防重放由 Coordination 平面跟踪）。
func NewNonce(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("primitives: nonce 长度必须 >0, got %d", n)
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
