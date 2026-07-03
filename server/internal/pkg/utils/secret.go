package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewSecretToken 生成密钥学安全的随机令牌（32 字节 = 64 hex 字符）。
//
// 替代原 NewULID()+NewULID() 拼接（重构清单 B-OTHER-11）：
// ULID 不是密钥学安全随机源，用于生成 AppSecret 等敏感凭证不安全。
// 本函数用 crypto/rand，是 Go 标准库的密钥学安全随机源。
func NewSecretToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败极其罕见（通常系统熵池枯竭），
		// 此时生成密钥不安全，必须 panic 而非返回弱密钥。
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}
