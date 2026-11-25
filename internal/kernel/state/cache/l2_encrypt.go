// l2_encrypt.go：PII 实体 L2 值加密（AES-256-GCM；contracts cache.pii.encrypt_l2）。
// 密钥经 WithL2EncryptKey 注入（装配从 env-only NETYADMIN_CACHE_L2_KEY 解码）；
// L1 保持明文（encrypt_l1: false——进程内存面，state-and-cache §5.7）。
// 密文格式 = nonce(12B) || GCM sealed（随机 nonce 每次写入；信封 Enc 标记）。
// 解密失败（密钥轮换/脏数据）按既有语义视为 miss——不返回错误（§5.6）。
package cache

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// l2EncryptKeySize 是 AES-256 密钥长度（与 GW AppSecret 同口径：32 字节）。
const l2EncryptKeySize = 32

// errDecryptL2 是密文解码失败哨兵（调用方按 miss 处理）。
var errDecryptL2 = errors.New("cache: decrypt L2 value")

// encryptL2Value 加密 L2 值（AES-256-GCM，随机 nonce 前缀）。
func encryptL2Value(key, plain []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

// decryptL2Value 解密 L2 值（nonce 前缀；失败返回 errDecryptL2）。
func decryptL2Value(key, sealed []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(sealed) < ns {
		return nil, errDecryptL2
	}
	plain, err := gcm.Open(nil, sealed[:ns], sealed[ns:], nil)
	if err != nil {
		return nil, errDecryptL2
	}
	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != l2EncryptKeySize {
		return nil, errDecryptL2
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
